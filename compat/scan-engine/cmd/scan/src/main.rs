mod cli;
mod run;

use clap::Parser;
use cli::Cli;

fn main() -> std::process::ExitCode {
    let cli = Cli::parse();
    match run::run(&cli) {
        Ok(()) => std::process::ExitCode::SUCCESS,
        Err(e) => {
            eprintln!("{e}");
            std::process::ExitCode::FAILURE
        }
    }
}
