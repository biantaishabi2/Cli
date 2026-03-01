//! BCC - Backend Compiler
//!
//! YAML to Elixir skeleton generator with extract, trace, and graph capabilities.

pub mod analyze;
pub mod arch;
pub mod arch_bridge_schema;
pub mod compile;
pub mod extract;
pub mod graph;
pub mod spec;
pub mod trace;

// Re-export score module for CLI integration
pub use arch::score;
