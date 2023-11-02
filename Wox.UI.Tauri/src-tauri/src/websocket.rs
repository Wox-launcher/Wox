use serde_json::Value;
use std::thread;
use std::time::Duration;
use tauri::{LogicalPosition, Position, Window};
use tungstenite::{connect, Message};
use url::Url;

use crate::get_server_port;

pub(crate) fn conn(window: Window) {
    let url = format!("ws://localhost:{}/ws", get_server_port());
    let url = Url::parse(&url).expect("Invalid URL");

    loop {
        match connect(url.clone()) {
            Ok((mut socket, _)) => {
                while let Ok(msg) = socket.read() {
                    if let Message::Text(msg) = msg {
                        handle_msg(&window, &msg);
                    }
                }
            }
            Err(e) => {
                info!("Error connecting to the server: {}", e);
                thread::sleep(Duration::from_secs(1));
            }
        }
    }
}

fn handle_msg(window: &Window, msg: &str) {
    if let Ok(v) = serde_json::from_str::<Value>(msg) {
        if let Some(method) = v.get("Method").and_then(|m| m.as_str()) {
            match method {
                "ToggleApp" => {
                    info!("tauri => ToggleApp");
                    if !window.is_visible().unwrap_or(false) {
                        if let Err(err) = handle_show_message(window, &v) {
                            info!("Error handling show message: {}", err);
                        }
                    } else {
                        if let Err(err) = window.hide() {
                            info!("Error hiding window: {}", err);
                        }
                    }
                }
                "ShowApp" => {
                    info!("tauri => ShowApp");
                    if let Err(err) = handle_show_message(window, &v) {
                        info!("Error handling show message: {}", err);
                    }
                }
                _ => {}
            }
        }
    }
}

fn handle_show_message(window: &Window, v: &Value) -> Result<(), Box<dyn std::error::Error>> {
    window.show()?;

    window.set_focus()?;
    // if windows is already shown, above code will not set focus on query box, so we need to set focus again
    window.eval(&format!("window.focus()"))?;

    if let Some(data) = v.get("Data") {
        if let Some(position) = data.get("Position") {
            if let (Some(x), Some(y), Some(type_position)) = (
                position.get("X").and_then(|x| x.as_f64()),
                position.get("Y").and_then(|y| y.as_f64()),
                position.get("Type").and_then(|t| t.as_str()),
            ) {
                if type_position == "MouseScreen" {
                    window.set_position(Position::Logical(LogicalPosition::new(x, y)))?;
                }
            }
        }
        if let Some(select_all) = data.get("SelectAll").and_then(|s| s.as_bool()) {
            if select_all {
                window.eval(&format!("window.selectAll()"))?;
            }
        }
    }
    Ok(())
}