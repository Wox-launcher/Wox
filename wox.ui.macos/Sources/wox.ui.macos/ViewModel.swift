import Foundation
import Combine
import SwiftUI
import AppKit

class WoxViewModel: ObservableObject {
    @Published var query: String = "" {
        didSet {
            if oldValue != query {
                sendQuery(text: query)
            }
        }
    }
    
    @Published var results: [WoxQueryResult] = []
    @Published var selectedResultId: String? {
        didSet {
            if oldValue != selectedResultId {
                onResultSelected()
            }
        }
    }
    @Published var isVisible: Bool = true
    
    // UI state for panels
    @Published var isShowActionPanel: Bool = false
    @Published var isShowPreviewPanel: Bool = false
    @Published var currentPreview: WoxPreview?
    @Published var toolbarInfo: ToolbarInfo = ToolbarInfo(text: nil, icon: nil, actions: nil)
    
    @Published var selectedActionId: String?
    
    var selectedResult: WoxQueryResult? {
        results.first(where: { $0.id == selectedResultId })
    }
    
    var selectedAction: WoxResultAction? {
        selectedResult?.actions?.first(where: { $0.id == selectedActionId })
    }
    
    private var currentQueryId: String = ""
    private var clearResultsTimer: Timer?
    
    let webSocketManager: WebSocketManager
    private var cancellables = Set<AnyCancellable>()
    
    init(port: Int) {
        self.webSocketManager = WebSocketManager(port: port)
        
        webSocketManager.messageReceived
            .receive(on: DispatchQueue.main)
            .sink { [weak self] msg in
                self?.handleMessage(msg)
            }
            .store(in: &cancellables)
    }
    
    func connect() {
        webSocketManager.connect()
    }
    
    func onUIReady() {
        let msg: [String: Any] = [
            "RequestId": UUID().uuidString,
            "TraceId": UUID().uuidString,
            "Type": "WebsocketMsgTypeRequest",
            "Method": "UIReady",
            "Data": [String: Any]()
        ]
        webSocketManager.sendRaw(message: msg)
    }
    
    private func sendQuery(text: String) {
        // Cancel existing timer
        clearResultsTimer?.invalidate()
        
        guard !text.isEmpty else {
            results = []
            selectedResultId = nil
            return
        }
        
        // Debounce clearing of results to avoid flickering
        // We set a timer to clear results after 150ms if no new results arrive
        clearResultsTimer = Timer.scheduledTimer(withTimeInterval: 0.15, repeats: false) { [weak self] _ in
            DispatchQueue.main.async {
                self?.results = []
                self?.selectedResultId = nil
            }
        }
        
        currentQueryId = UUID().uuidString
        
        let msg: [String: Any] = [
            "RequestId": UUID().uuidString,
            "TraceId": UUID().uuidString,
            "Type": "WebsocketMsgTypeRequest",
            "Method": "Query",
            "Data": [
                "queryId": currentQueryId,
                "queryType": "input",
                "queryText": text,
                "querySelection": ["type": "", "text": "", "filePaths": [String]()]
            ]
        ]
        
        print("[Query] Sending query: \(text), queryId: \(currentQueryId)")
        webSocketManager.sendRaw(message: msg)
    }
    
    func executeAction(result: WoxQueryResult, actionId: String? = nil) {
        let action: WoxResultAction?
        if let aid = actionId {
            action = result.actions?.first(where: { $0.id == aid })
        } else {
            action = result.actions?.first(where: { $0.isDefault }) ?? result.actions?.first
        }
        
        guard let targetAction = action else {
            print("[Action] No action found for result: \(result.title)")
            return
        }
        
        let msg: [String: Any] = [
            "RequestId": UUID().uuidString,
            "TraceId": UUID().uuidString,
            "Type": "WebsocketMsgTypeRequest",
            "Method": "Action",
            "Data": [
                "resultId": result.id,
                "actionId": targetAction.id
            ]
        ]
        
        print("[Action] Executing action: \(targetAction.name) for result: \(result.title)")
        webSocketManager.sendRaw(message: msg)
        
        // Hide app after action
        hideApp()
    }
    
    func toggleActionPanel() {
        guard let result = selectedResult, result.actions != nil else { return }
        isShowActionPanel.toggle()
        if isShowActionPanel {
            selectedActionId = result.actions?.first(where: { $0.isDefault })?.id ?? result.actions?.first?.id
        }
    }
    
    func hideApp() {
        isVisible = false
        query = ""
        results = []
        selectedResultId = nil
        NSApp.hide(nil)
    }
    
    func showApp() {
        isVisible = true
        NSApp.activate(ignoringOtherApps: true)
        if let window = NSApp.windows.first {
            window.makeKeyAndOrderFront(nil)
        }
    }
    
    private func handleMessage(_ json: [String: Any]) {
        // Flutter uses PascalCase: Type, Method, RequestId, TraceId, Data
        let type = json["Type"] as? String
        let method = json["Method"] as? String
        
        guard let type = type, let method = method else {
            // Print what we got to help debug
            let keys = json.keys.joined(separator: ", ")
            print("[WS] Invalid message format. Keys: \(keys)")
            return
        }
        
        print("[WS] Received message: Type=\(type), Method=\(method)")
        
        if type == "WebsocketMsgTypeResponse" {
            if method == "Query" {
                handleQueryResponse(json)
            }
        } else if type == "WebsocketMsgTypeRequest" {
            handleServerRequest(method: method, json: json)
        }
    }
    
    private func onResultSelected() {
        guard let result = selectedResult else {
            isShowPreviewPanel = false
            currentPreview = nil
            updateToolbarWithActions(actions: [])
            return
        }
        
        // Update Preview
        if let preview = result.preview, !preview.previewData.isEmpty {
            currentPreview = preview
            isShowPreviewPanel = true
        } else {
            currentPreview = nil
            isShowPreviewPanel = false
        }
        
        // Update Toolbar Actions
        updateToolbarWithActions(actions: result.actions ?? [])
    }
    
    private func updateToolbarWithActions(actions: [WoxResultAction]) {
        // Filter actions that have hotkeys
        var actionsWithHotkeys = actions.filter { !$0.hotkey.isEmpty }
        
        // Sort: non-default first, default at the end
        actionsWithHotkeys.sort { a, b in
            if a.isDefault && !b.isDefault { return false }
            if !a.isDefault && b.isDefault { return true }
            return false
        }
        
        let toolbarActions = actionsWithHotkeys.map { 
            ToolbarActionInfo(name: $0.name, hotkey: $0.hotkey) 
        }
        
        // Add "More Actions" cmd+j
        var finalActions = toolbarActions
        if !actions.isEmpty {
            finalActions.append(ToolbarActionInfo(name: "More Actions", hotkey: "cmd+j"))
        }
        
        toolbarInfo = ToolbarInfo(
            text: toolbarInfo.text,
            icon: toolbarInfo.icon,
            actions: finalActions.isEmpty ? nil : finalActions
        )
    }
    
    private func handleServerRequest(method: String, json: [String: Any]) {
        switch method {
        case "ShowApp":
            showApp()
            sendResponse(to: json, success: true, data: nil)
            
        case "HideApp":
            hideApp()
            sendResponse(to: json, success: true, data: nil)
            
        case "ChangeQuery":
            if let data = json["Data"] as? [String: Any],
               let queryText = data["queryText"] as? String {
                query = queryText
            }
            sendResponse(to: json, success: true, data: nil)
            
        case "ToggleApp":
            if isVisible {
                hideApp()
            } else {
                showApp()
            }
            sendResponse(to: json, success: true, data: nil)
            
        case "SetToolbar":
            if let data = json["Data"] as? [String: Any] {
                let text = data["text"] as? String
                let icon = data["icon"] as? String
                toolbarInfo = ToolbarInfo(text: text, icon: icon, actions: toolbarInfo.actions)
            }
            sendResponse(to: json, success: true, data: nil)
            
        default:
            print("[WS] Unknown server request: \(method)")
            sendResponse(to: json, success: true, data: nil)
        }
    }
    
    private func sendResponse(to request: [String: Any], success: Bool, data: Any?) {
        guard let requestId = request["RequestId"] as? String,
              let traceId = request["TraceId"] as? String,
              let method = request["Method"] as? String else {
            return
        }
        
        var response: [String: Any] = [
            "RequestId": requestId,
            "TraceId": traceId,
            "Type": "WebsocketMsgTypeResponse",
            "Method": method,
            "Success": success
        ]
        
        if let data = data {
            response["Data"] = data
        } else {
            response["Data"] = [String: Any]()
        }
        
        webSocketManager.sendRaw(message: response)
    }
    
    private func handleQueryResponse(_ json: [String: Any]) {
        // Response format: {"Data": {"Results": [...], "IsFinal": bool}}
        guard let data = json["Data"] as? [String: Any],
              let resultsArray = data["Results"] as? [[String: Any]] else {
            print("[Query] No Results in response. Data: \(json["Data"] ?? "nil")")
            return
        }
        
        let isFinal = data["IsFinal"] as? Bool ?? false
        print("[Query] Received \(resultsArray.count) results, isFinal: \(isFinal)")
        
        // Cancel the clear timer as we have incoming results
        clearResultsTimer?.invalidate()
        clearResultsTimer = nil
        
        var parsedResults: [WoxQueryResult] = []
        
        for resultDict in resultsArray {
            if let result = parseQueryResult(resultDict) {
                parsedResults.append(result)
            } else {
                print("[Query] Failed to parse result: \(resultDict.keys.joined(separator: ", "))")
            }
        }
        
        print("[Query] Parsed \(parsedResults.count) results")
        
        // Append results (they come in batches)
        if !parsedResults.isEmpty {
            // Merge with existing results
            let existingIds = Set(results.map { $0.id })
            let newResults = parsedResults.filter { !existingIds.contains($0.id) }
            
            print("[Query] Adding \(newResults.count) new results (existing: \(results.count))")
            
            results.append(contentsOf: newResults)
            
            // Sort by score (descending)
            results.sort { ($0.score) > ($1.score) }
            
            // Select first result if none selected
            if selectedResultId == nil && !results.isEmpty {
                selectedResultId = results.first?.id
                print("[Query] Selected first result: \(results.first?.title ?? "none")")
            }
        }
    }
    
    private func parseQueryResult(_ dict: [String: Any]) -> WoxQueryResult? {
        guard let id = dict["Id"] as? String,
              let title = dict["Title"] as? String else {
            return nil
        }
        
        let subTitle = dict["SubTitle"] as? String
        let score = dict["Score"] as? Int ?? 0
        let group = dict["Group"] as? String
        let queryId = dict["QueryId"] as? String
        
        // Parse icon - format: {"ImageType": "xxx", "ImageData": "xxx"}
        var icon: WoxIcon? = nil
        if let iconDict = dict["Icon"] as? [String: Any],
           let imageType = iconDict["ImageType"] as? String,
           let imageData = iconDict["ImageData"] as? String {
            icon = WoxIcon(imageType: imageType, imageData: imageData)
        }
        
        // Parse actions
        var actions: [WoxResultAction] = []
        if let actionsArray = dict["Actions"] as? [[String: Any]] {
            for actionDict in actionsArray {
                if let actionId = actionDict["Id"] as? String,
                   let name = actionDict["Name"] as? String {
                    let isDefault = actionDict["IsDefault"] as? Bool ?? false
                    let hotkey = actionDict["Hotkey"] as? String ?? ""
                    actions.append(WoxResultAction(id: actionId, name: name, isDefault: isDefault, hotkey: hotkey))
                }
            }
        }
        
        // Parse preview
        var preview: WoxPreview? = nil
        if let previewDict = dict["Preview"] as? [String: Any],
           let previewType = previewDict["PreviewType"] as? String,
           let previewData = previewDict["PreviewData"] as? String {
            let properties = previewDict["PreviewProperties"] as? [String: String] ?? [:]
            preview = WoxPreview(previewType: previewType, previewData: previewData, previewProperties: properties)
        }
        
        // Parse tails
        var tails: [WoxListItemTail] = []
        if let tailsArray = dict["Tails"] as? [[String: Any]] {
            for tailDict in tailsArray {
                if let type = tailDict["Type"] as? String {
                    let text = tailDict["Text"] as? String
                    var tailIcon: WoxIcon? = nil
                    if let iconDict = tailDict["Icon"] as? [String: Any],
                       let it = iconDict["ImageType"] as? String,
                       let idData = iconDict["ImageData"] as? String {
                        tailIcon = WoxIcon(imageType: it, imageData: idData)
                    }
                    tails.append(WoxListItemTail(type: type, text: text, icon: tailIcon))
                }
            }
        }
        
        return WoxQueryResult(
            id: id,
            title: title,
            subTitle: subTitle,
            icon: icon,
            score: score,
            group: group,
            actions: actions.isEmpty ? nil : actions,
            preview: preview,
            tails: tails.isEmpty ? nil : tails,
            queryId: queryId
        )
    }
}
