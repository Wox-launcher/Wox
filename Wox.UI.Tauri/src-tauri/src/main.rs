// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::env;

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

fn main() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![get_server_port])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
