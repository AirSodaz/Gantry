package tasks

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AirSodaz/gantry/internal/taskmessage"
)

const (
	contentSegmentMaxBytes = 256 << 10
	contentSegmentMaxAge   = time.Second
)

type contentStream struct {
	mu    sync.Mutex
	data  []byte
	timer *time.Timer
}

type modelDelta struct {
	StreamID      string `json:"stream_id"`
	ProtoStreamID string `json:"streamId"`
	Text          string `json:"text"`
}

func (s *Service) appendModelDelta(ctx context.Context, runID, payload string) error {
	var delta modelDelta
	if err := json.Unmarshal([]byte(payload), &delta); err != nil {
		return ErrInvalidInput
	}
	if delta.StreamID == "" {
		delta.StreamID = delta.ProtoStreamID
	}
	if delta.StreamID == "" {
		delta.StreamID = "model"
	}
	if delta.Text == "" {
		return nil
	}

	key := contentStreamKey{runID: runID, streamID: delta.StreamID}
	stream := s.contentStream(key)
	stream.mu.Lock()
	defer stream.mu.Unlock()
	stream.data = append(stream.data, delta.Text...)
	if len(stream.data) >= contentSegmentMaxBytes {
		if stream.timer != nil {
			stream.timer.Stop()
			stream.timer = nil
		}
		for len(stream.data) >= contentSegmentMaxBytes {
			if err := s.flushContentStreamLocked(ctx, key, stream); err != nil {
				return err
			}
		}
	}
	if stream.timer == nil {
		stream.timer = time.AfterFunc(contentSegmentMaxAge, func() {
			_ = s.flushContentStream(context.Background(), key)
		})
	}
	return nil
}

func (s *Service) contentStream(key contentStreamKey) *contentStream {
	s.content.mu.Lock()
	defer s.content.mu.Unlock()
	if s.content.streams == nil {
		s.content.streams = make(map[contentStreamKey]*contentStream)
	}
	stream := s.content.streams[key]
	if stream == nil {
		stream = &contentStream{}
		s.content.streams[key] = stream
	}
	return stream
}

func (s *Service) flushRunContent(ctx context.Context, runID string) error {
	s.content.mu.Lock()
	keys := make([]contentStreamKey, 0)
	for key := range s.content.streams {
		if key.runID == runID {
			keys = append(keys, key)
		}
	}
	s.content.mu.Unlock()
	for _, key := range keys {
		if err := s.flushContentStream(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) flushContentStream(ctx context.Context, key contentStreamKey) error {
	stream := s.contentStream(key)
	stream.mu.Lock()
	defer stream.mu.Unlock()
	return s.flushContentStreamLocked(ctx, key, stream)
}

func (s *Service) flushContentStreamLocked(ctx context.Context, key contentStreamKey, stream *contentStream) error {
	if stream.timer != nil {
		stream.timer.Stop()
		stream.timer = nil
	}
	if len(stream.data) == 0 {
		return nil
	}
	size := len(stream.data)
	if size > contentSegmentMaxBytes {
		size = contentSegmentMaxBytes
	}
	data := append([]byte(nil), stream.data[:size]...)
	stream.data = stream.data[size:]
	if err := s.persistContentSegment(ctx, key.runID, key.streamID, data); err != nil {
		stream.data = append(data, stream.data...)
		return err
	}
	return nil
}

func (s *Service) persistContentSegment(ctx context.Context, runID, streamID string, data []byte) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var start int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(end_offset),0) FROM gantry.run_content_segments WHERE run_id=$1 AND stream_id=$2`, runID, streamID).Scan(&start); err != nil {
		return err
	}
	end := start + int64(len(data))
	segmentID := newID("seg")
	objectKey := "segments/" + runID + "/" + streamID + "/" + strconv.FormatInt(start, 10)
	digest := sha256.Sum256(data)
	if err := s.store.Put(ctx, objectKey, bytes.NewReader(data), int64(len(data)), "text/plain; charset=utf-8"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.run_content_segments (id, run_id, stream_id, start_offset, end_offset, object_key, digest, size_bytes, media_type) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, segmentID, runID, streamID, start, end, objectKey, "sha256:"+hex.EncodeToString(digest[:]), len(data), "text/plain; charset=utf-8"); err != nil {
		return err
	}
	var taskID string
	if err := tx.QueryRow(ctx, `SELECT task_id FROM gantry.runs WHERE id=$1`, runID).Scan(&taskID); err != nil {
		return err
	}
	if err := taskmessage.Append(ctx, tx, taskID, runID, "agent", taskmessage.Text(string(data))); err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{"segment_id": segmentID, "stream_id": streamID, "start_offset": start, "end_offset": end})
	if err := appendEventPayload(ctx, tx, runID, "model.segment", string(payload)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) hydrateContentSegment(ctx context.Context, event *Event) error {
	if event.Type != "model.segment" || s.store == nil {
		return nil
	}
	var reference struct {
		SegmentID string `json:"segment_id"`
		StreamID  string `json:"stream_id"`
	}
	if err := json.Unmarshal(event.Payload, &reference); err != nil || reference.SegmentID == "" {
		return ErrInvalidInput
	}
	var objectKey, digest string
	var size int64
	if err := s.pool.QueryRow(ctx, `SELECT object_key, digest, size_bytes FROM gantry.run_content_segments WHERE id=$1`, reference.SegmentID).Scan(&objectKey, &digest, &size); err != nil {
		return err
	}
	body, err := s.store.Get(ctx, objectKey)
	if err != nil {
		return err
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, size+1))
	if err != nil || int64(len(data)) != size {
		return ErrInvalidInput
	}
	sum := sha256.Sum256(data)
	if digest != "sha256:"+hex.EncodeToString(sum[:]) {
		return ErrInvalidInput
	}
	event.Type = "model.delta"
	event.Payload, _ = json.Marshal(map[string]string{"stream_id": strings.TrimSpace(reference.StreamID), "text": string(data)})
	return nil
}
