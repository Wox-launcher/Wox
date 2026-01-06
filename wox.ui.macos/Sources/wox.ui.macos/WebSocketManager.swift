import Foundation
import Combine
import Starscream

class WebSocketManager: ObservableObject, WebSocketDelegate {
    private var socket: WebSocket?
    private let url: URL
    
    // Publisher for UI to subscribe to
    let messageReceived = PassthroughSubject<[String: Any], Never>()
    let isConnected = CurrentValueSubject<Bool, Never>(false)
    
    init(port: Int) {
        self.url = URL(string: "ws://127.0.0.1:\(port)/ws")!
    }
    
    func connect() {
        disconnect()
        
        var request = URLRequest(url: url)
        request.timeoutInterval = 5
        
        socket = WebSocket(request: request)
        socket?.delegate = self
        socket?.connect()
        
        print("[WS] Connecting to \(url)")
    }
    
    func disconnect() {
        socket?.disconnect()
        socket = nil
        isConnected.send(false)
    }
    
    func sendRaw(message: [String: Any]) {
        guard let sock = socket, isConnected.value else {
            print("[WS] Cannot send: no connection")
            return
        }
        
        do {
            let data = try JSONSerialization.data(withJSONObject: message, options: [])
            guard let jsonString = String(data: data, encoding: .utf8) else {
                print("[WS] Cannot convert data to string")
                return
            }
            
            print("[WS] Sending: \(jsonString.prefix(200))...")
            sock.write(string: jsonString)
        } catch {
            print("[WS] Error encoding message: \(error)")
        }
    }
    
    // MARK: - WebSocketDelegate
    
    func didReceive(event: Starscream.WebSocketEvent, client: any Starscream.WebSocketClient) {
        switch event {
        case .connected(let headers):
            print("[WS] Connected, headers: \(headers)")
            DispatchQueue.main.async {
                self.isConnected.send(true)
            }
            
        case .disconnected(let reason, let code):
            print("[WS] Disconnected: \(reason), code: \(code)")
            DispatchQueue.main.async {
                self.isConnected.send(false)
            }
            
        case .text(let text):
            handleMessage(text)
            
        case .binary(let data):
            if let text = String(data: data, encoding: .utf8) {
                handleMessage(text)
            }
            
        case .error(let error):
            print("[WS] Error: \(error?.localizedDescription ?? "unknown")")
            DispatchQueue.main.async {
                self.isConnected.send(false)
            }
            
        case .cancelled:
            print("[WS] Cancelled")
            DispatchQueue.main.async {
                self.isConnected.send(false)
            }
            
        case .viabilityChanged(let viable):
            print("[WS] Viability changed: \(viable)")
            
        case .reconnectSuggested(let suggested):
            print("[WS] Reconnect suggested: \(suggested)")
            if suggested {
                connect()
            }
            
        case .ping, .pong, .peerClosed:
            break
        }
    }
    
    private func handleMessage(_ text: String) {
        guard let data = text.data(using: .utf8) else { return }
        
        do {
            if let json = try JSONSerialization.jsonObject(with: data) as? [String: Any] {
                DispatchQueue.main.async {
                    self.messageReceived.send(json)
                }
            }
        } catch {
            print("[WS] Error decoding message: \(error). Text: \(text.prefix(200))")
        }
    }
}
