pub mod types;

pub mod gantry {
    pub mod runner {
        pub mod v1 {
            tonic::include_proto!("gantry.runner.v1");
        }
    }
}
