fn main() -> Result<(), Box<dyn std::error::Error>> {
    let protoc = protoc_bin_vendored::protoc_bin_path()?;
    unsafe {
        std::env::set_var("PROTOC", protoc);
    }

    tonic_build::configure()
        .build_server(false)
        .compile_protos(
            &["../../../proto/gantry/runner/v1/runner.proto"],
            &["../../../proto"],
        )?;

    println!("cargo:rerun-if-changed=../../../proto/gantry/runner/v1/runner.proto");
    Ok(())
}
