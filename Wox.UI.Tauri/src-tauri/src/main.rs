// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

#[macro_use]
extern crate log;
extern crate simplelog;

use simplelog::*;
use std::env;
use std::fs::File;
use std::path::PathBuf;
use tauri::Manager;
use tauri_nspanel::{panel_delegate, WindowExt};
use tauri_nspanel::cocoa::appkit::{NSMainMenuWindowLevel, NSWindowCollectionBehavior};

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
        .plugin(tauri_nspanel::init())
        .invoke_handler(tauri::generate_handler![get_server_port, log_ui])
        .setup(|app| {
            if cfg!(target_os = "macos") {
                // hide the dock icon
                app.set_activation_policy(tauri::ActivationPolicy::Accessory);

                let window = app.get_window("main").unwrap();
                let panel = window.to_panel().unwrap();
                // Set panel above the main menu window level
                panel.set_level(NSMainMenuWindowLevel + 1);
                // Ensure that the panel can display over the top of fullscreen apps
                panel.set_collection_behaviour(NSWindowCollectionBehavior::NSWindowCollectionBehaviorTransient
                    | NSWindowCollectionBehavior::NSWindowCollectionBehaviorMoveToActiveSpace
                );
            }

            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}