// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

#[macro_use]
extern crate log;
extern crate simplelog;

use std::env;
use std::path::PathBuf;
use simplelog::*;
use std::fs::File;

#[tauri::command]
fn get_server_port() -> String {
    let args: Vec<String> = env::args().collect();
    // use default port 34987 if args[1] is not provided
    let port = if args.len() > 1 {
        args[1].parse::<u16>().unwrap_or(34987)
    } else {
        34987
    };
    port.to_string()
}

#[tauri::command]
fn log_ui(msg: String) {
    info!("{}", msg)
}

fn init_log_file() {
    if let Some(home_dir) = dirs::home_dir() {
       let mut base_path = PathBuf::new();
       base_path.push(home_dir);
       base_path.push(".wox");
       base_path.push("log");
       base_path.push("ui.log");
       CombinedLogger::init(
               vec![
                   TermLogger::new(LevelFilter::Warn, Config::default(), TerminalMode::Mixed, ColorChoice::Auto),
                   WriteLogger::new(LevelFilter::Info, Config::default(), File::create(base_path).unwrap()),
               ]
           ).unwrap();
   } else {
       println!("Can not find user main home path");
   }
}

fn main() {
    init_log_file();
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![get_server_port, log_ui])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
