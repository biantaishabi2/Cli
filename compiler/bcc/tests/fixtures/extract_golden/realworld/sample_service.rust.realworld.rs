use std::collections::HashMap;

pub struct Config {
    pub name: String,
}

pub fn run(config: &Config) {
    let _ = HashMap::<String, String>::new();
    println!("{}", config.name);
}
