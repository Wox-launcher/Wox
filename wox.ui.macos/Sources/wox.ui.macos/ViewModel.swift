import AppKit
import Combine
import Foundation
import SwiftUI

// MARK: - ViewModel

class WoxViewModel: ObservableObject {
    @Published var query: String = "" {
        didSet {
            if oldValue != query {
                updateQueryBoxLineCount(query)
                sendQuery(text: query)
            }
        }
    }

    @Published var currentQuery: PlainQuery = PlainQuery.empty()
    
    let resultListController = ResultListController()
    let resultGridController = ResultGridController()
    let actionListController = ActionListController()
    
    var activeResultController: Any {
        if isGridLayout {
            return resultGridController
        } else {
            return resultListController
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

    @Published var isShowActionPanel: Bool = false
    @Published var isShowPreviewPanel: Bool = false
    @Published var isShowFormActionPanel: Bool = false
    @Published var currentPreview: WoxPreview?
    @Published var toolbarInfo: ToolbarInfo = ToolbarInfo(text: nil, icon: nil, actions: nil)

    @Published var selectedActionId: String?
    @Published var activeFormAction: WoxResultAction?
    @Published var formActionValues: [String: Any] = [:]

    @Published var theme: WoxTheme = WoxTheme.empty()
    @Published var queryBoxLineCount: Int = 1
    @Published var activeResultIndex: Int = 0

    // MARK: - Quick Select Mode
    @Published var isQuickSelectMode: Bool = false
    private var quickSelectTimer: Timer?
    private let quickSelectDelay: TimeInterval = 0.3

    // MARK: - Loading Animation
    @Published var isLoading: Bool = false
    private var loadingTimer: Timer?
    private let loadingDelay: TimeInterval = 0.5

    // MARK: - Query Icon
    @Published var queryIcon: QueryIconInfo = QueryIconInfo.empty()

    // MARK: - Grid Layout
    @Published var isGridLayout: Bool = false
    @Published var gridLayoutParams: GridLayoutParams = GridLayoutParams.empty()

    private var _pendingPreservedIndex: Int?
    var pendingPreservedIndex: Int? {
        get { _pendingPreservedIndex }
        set { _pendingPreservedIndex = newValue }
    }

    var selectedResult: WoxQueryResult? {
        results.first(where: { $0.id == selectedResultId })
    }

    var selectedAction: WoxResultAction? {
        selectedResult?.actions?.first(where: { $0.id == selectedActionId })
    }

    private var currentQueryId: String = ""
    private var clearResultsTimer: Timer?
    private var maxResultCount: Int = 10
    private let clearResultsDelay: TimeInterval = 0.15

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

        loadTheme()
    }

    func connect() {
        webSocketManager.connect()
    }

    func onUIReady() {
        let msg = WoxWebsocketMsg(method: .uiReady)
        webSocketManager.send(message: msg)
    }

    func loadTheme() {
        let msg = WoxWebsocketMsg(method: .theme)
        webSocketManager.send(message: msg)
    }

    private func updateQueryBoxLineCount(_ text: String) {
        let lines = text.components(separatedBy: .newlines).count
        let clampedLines = max(1, min(lines, QUERY_BOX_MAX_LINES))
        if clampedLines != queryBoxLineCount {
            queryBoxLineCount = clampedLines
        }
    }

    private func sendQuery(text: String) {
        guard !text.isEmpty else {
            clearResultsTimer?.invalidate()
            clearResultsTimer = nil
            results = []
            selectedResultId = nil
            
            resultListController.updateItems([])
            resultGridController.updateItems([])
            return
        }

        clearResultsTimer?.invalidate()

        currentQueryId = UUID().uuidString
        currentQuery = PlainQuery(
            queryId: currentQueryId,
            queryType: "input",
            queryText: text,
            querySelection: Selection.empty()
        )

        let msg = WoxWebsocketMsg(
            method: .query,
            data: [
                "queryId": currentQueryId,
                "queryType": "input",
                "queryText": text,
                "querySelection": ["type": "", "text": "", "filePaths": [String]()],
            ] as [String: Any],
            sendTimestamp: Int(Date().timeIntervalSince1970 * 1000)
        )

        webSocketManager.send(message: msg)

        // Start loading timer
        startLoadingTimer()

        clearResultsTimer = Timer.scheduledTimer(
            withTimeInterval: clearResultsDelay, repeats: false
        ) { [weak self] _ in
            DispatchQueue.main.async {
                self?.results = []
                self?.selectedResultId = nil
                self?.resultListController.updateItems([])
                self?.resultGridController.updateItems([])
            }
        }
    }

    private func handleQueryResponse(_ json: [String: Any]) {
        guard let data = json["Data"] as? [String: Any],
            let resultsArray = data["Results"] as? [[String: Any]]
        else {
            return
        }

        clearResultsTimer?.invalidate()
        clearResultsTimer = nil

        var parsedResults: [WoxQueryResult] = []
        for resultDict in resultsArray {
            if let result = WoxQueryResult.fromJson(resultDict) {
                parsedResults.append(result)
            }
        }

        // Extract QueryId from Data if available, otherwise fallback to first result's QueryId or empty
        let queryId = data["QueryId"] as? String ?? parsedResults.first?.queryId ?? ""

        if !parsedResults.isEmpty {
            onReceivedQueryResults(parsedResults, queryId: queryId)
        }
    }

    // MARK: - Query MRU (Most Recently Used)

    func queryMRU() {
        let queryId = UUID().uuidString
        currentQuery = PlainQuery(
            queryId: queryId,
            queryType: "input",
            queryText: "",
            querySelection: Selection.empty()
        )

        let msg = WoxWebsocketMsg(
            method: .queryMRU,
            data: ["queryId": queryId] as [String: Any]
        )

        webSocketManager.send(message: msg)
        
        resultListController.updateItems([])
        resultGridController.updateItems([])
    }

    private func onReceivedQueryResults(_ receivedResults: [WoxQueryResult], queryId: String) {
        // Stop loading animation
        stopLoading()

        if receivedResults.isEmpty {
            return
        }

        // Use passed queryId for verification if available
        let checkQueryId = queryId.isEmpty ? (receivedResults.first?.queryId ?? "") : queryId

        if checkQueryId != currentQuery.queryId {
            return
        }

        let existingQueryResults = results.filter { $0.queryId == currentQuery.queryId }
        var finalResults = existingQueryResults + receivedResults

        for i in 0..<finalResults.count {
            if let actions = finalResults[i].actions,
                let defaultActionIndex = actions.firstIndex(where: { $0.isDefault }),
                defaultActionIndex != 0
            {
                let defaultAction = actions[defaultActionIndex]
                var newActions = actions
                newActions.remove(at: defaultActionIndex)
                newActions.insert(defaultAction, at: 0)
                finalResults[i].actions = newActions
            }
        }

        let finalResultsSorted = groupAndSortResults(finalResults)
        results = finalResultsSorted
        
        let listItems = finalResultsSorted.map { WoxListItem.fromQueryResult($0) }
        resultListController.updateItems(listItems, silent: true)
        resultGridController.updateItems(listItems, silent: true)

        if let preservedIndex = pendingPreservedIndex {
            let activeResults = results.filter { !$0.isGroup }
            if preservedIndex < activeResults.count {
                selectedResultId = activeResults[preservedIndex].id
                activeResultIndex = results.firstIndex(where: { $0.id == selectedResultId }) ?? 0
                
                resultListController.updateActiveIndex(activeResultIndex)
                resultGridController.updateActiveIndex(activeResultIndex)
            } else {
                resetActiveResult()
            }
            pendingPreservedIndex = nil
        } else {
            if existingQueryResults.isEmpty || activeResultIndex == 0 {
                resetActiveResult()
            }
        }
    }

    private func groupAndSortResults(_ results: [WoxQueryResult]) -> [WoxQueryResult] {
        var groupScoreMap: [String: Int] = [:]
        for result in results {
            if groupScoreMap[result.group] == nil {
                groupScoreMap[result.group] = result.groupScore
            }
        }

        var groups = Array(Set(results.map { $0.group }))
        groups.sort { groupScoreMap[$0, default: 0] > groupScoreMap[$1, default: 0] }

        var finalResultsSorted: [WoxQueryResult] = []
        for group in groups {
            let groupResults = results.filter { $0.group == group }
            let groupResultsSorted = groupResults.sorted { $0.score > $1.score }

            if !group.isEmpty {
                let groupHeader = WoxQueryResult(
                    id: "group_\(group)",
                    queryId: results.first?.queryId ?? "",
                    title: group,
                    subTitle: "",
                    icon: nil,
                    score: groupResultsSorted.first?.groupScore ?? 0,
                    group: group,
                    groupScore: groupResultsSorted.first?.groupScore ?? 0,
                    isGroup: true,
                    preview: nil,
                    tails: nil,
                    actions: nil
                )
                finalResultsSorted.append(groupHeader)
            }

            finalResultsSorted.append(contentsOf: groupResultsSorted)
        }

        return finalResultsSorted
    }

    func executeAction(result: WoxQueryResult, actionId: String? = nil) {
        let action: WoxResultAction?
        if let aid = actionId {
            action = result.actions?.first(where: { $0.id == aid })
        } else {
            action = result.actions?.first(where: { $0.isDefault }) ?? result.actions?.first
        }

        guard let targetAction = action else { return }

        // Handle form type action - show form panel instead of executing
        if targetAction.type == "form" {
            showFormActionPanel(action: targetAction, resultId: result.id)
            return
        }

        let msg = WoxWebsocketMsg(
            method: .action,
            data: [
                "resultId": result.id,
                "actionId": targetAction.id,
                "queryId": result.queryId,
            ] as [String: Any]
        )

        webSocketManager.send(message: msg)

        // Only hide app if preventHideAfterAction is false
        if !targetAction.preventHideAfterAction {
            hideApp()
        }

        // Hide action panel after execution
        isShowActionPanel = false
    }

    func toggleActionPanel() {
        guard let result = selectedResult, result.actions != nil else { return }
        isShowActionPanel.toggle()
        if isShowActionPanel {
            selectedActionId =
                result.actions?.first(where: { $0.isDefault })?.id ?? result.actions?.first?.id
        }
    }

    // MARK: - Form Action Panel

    @Published var activeFormResultId: String = ""

    func showFormActionPanel(action: WoxResultAction, resultId: String) {
        activeFormAction = action
        activeFormResultId = resultId
        formActionValues = [:]

        // Initialize with default values from form
        if let form = action.form {
            for item in form {
                // Simple key extraction - in real implementation, parse JSON value
                let key = item.value
                if !key.isEmpty {
                    formActionValues[key] = ""
                }
            }
        }

        isShowFormActionPanel = true
        isShowActionPanel = false
    }

    func hideFormActionPanel() {
        activeFormAction = nil
        activeFormResultId = ""
        formActionValues = [:]
        isShowFormActionPanel = false
    }

    func submitFormAction(values: [String: String]) {
        guard let action = activeFormAction, !activeFormResultId.isEmpty else {
            hideFormActionPanel()
            return
        }

        let msg = WoxWebsocketMsg(
            method: .formAction,
            data: [
                "resultId": activeFormResultId,
                "actionId": action.id,
                "queryId": currentQuery.queryId,
                "values": values,
            ] as [String: Any]
        )

        webSocketManager.send(message: msg)
        hideFormActionPanel()
    }

    // MARK: - Hotkey Action Execution

    /// Parse hotkey string like "cmd+k" to modifiers and key
    private func parseHotkey(_ hotkeyStr: String) -> (
        modifiers: NSEvent.ModifierFlags, key: String
    )? {
        guard !hotkeyStr.isEmpty else { return nil }

        let parts = hotkeyStr.lowercased().split(separator: "+").map { String($0) }
        guard !parts.isEmpty else { return nil }

        var modifiers: NSEvent.ModifierFlags = []
        var key = ""

        for part in parts {
            switch part {
            case "cmd", "command":
                modifiers.insert(.command)
            case "ctrl", "control":
                modifiers.insert(.control)
            case "alt", "option":
                modifiers.insert(.option)
            case "shift":
                modifiers.insert(.shift)
            default:
                key = part
            }
        }

        return key.isEmpty ? nil : (modifiers, key)
    }

    /// Get action by hotkey from the selected result
    func getActionByHotkey(modifiers: NSEvent.ModifierFlags, key: String) -> WoxResultAction? {
        guard let result = selectedResult, let actions = result.actions else { return nil }

        for action in actions {
            if let parsed = parseHotkey(action.hotkey) {
                if parsed.modifiers == modifiers && parsed.key == key.lowercased() {
                    return action
                }
            }
        }

        return nil
    }

    /// Execute action by hotkey, returns true if action was found and executed
    func executeActionByHotkey(modifiers: NSEvent.ModifierFlags, key: String) -> Bool {
        guard let result = selectedResult else { return false }

        if let action = getActionByHotkey(modifiers: modifiers, key: key) {
            executeAction(result: result, actionId: action.id)
            return true
        }

        return false
    }

    /// Handle keyboard event for action hotkeys
    func handleKeyboardEvent(modifiers: NSEvent.ModifierFlags, key: String) -> Bool {
        // Skip if action panel is shown - let action panel handle it
        if isShowActionPanel { return false }

        // Check for action hotkeys
        return executeActionByHotkey(modifiers: modifiers, key: key)
    }

    // MARK: - Auto Complete

    /// Auto complete query with the selected result's title
    func autoCompleteQuery() {
        if isShowActionPanel {
            // Action panel auto-complete if needed, but Flutter only does it for main results
            return
        }
        
        let controller = isGridLayout ? resultGridController : resultListController
        guard controller.activeIndex < controller.items.count else { return }
        let result = controller.items[controller.activeIndex]
        if !result.isGroup {
            query = result.title
        }
    }

    private func getQueryBoxTotalHeight() -> CGFloat {
        let extraLines = CGFloat(queryBoxLineCount - 1)
        return (QUERY_BOX_BASE_HEIGHT + theme.appPaddingTop + theme.appPaddingBottom)
            + (QUERY_BOX_LINE_HEIGHT * extraLines)
    }

    func calculateTotalHeight() -> CGFloat {
        var height = getQueryBoxTotalHeight()

        let itemCount = results.count
        if itemCount > 0 {
            let visibleCount = min(results.filter { !$0.isGroup }.count, maxResultCount)
            let itemHeight =
                RESULT_ITEM_BASE_HEIGHT + theme.resultItemPaddingTop + theme.resultItemPaddingBottom
            height += CGFloat(visibleCount) * itemHeight
            height += theme.resultContainerPaddingTop + theme.resultContainerPaddingBottom
        }

        if isShowActionPanel || isShowPreviewPanel || isShowFormActionPanel {
            let itemHeight =
                RESULT_ITEM_BASE_HEIGHT + theme.resultItemPaddingTop + theme.resultItemPaddingBottom
            height =
                getQueryBoxTotalHeight() + (CGFloat(maxResultCount) * itemHeight)
                + theme.resultContainerPaddingTop + theme.resultContainerPaddingBottom
        }

        if isShowToolbar {
            height += TOOLBAR_HEIGHT
        }

        return height
    }

    var isShowToolbar: Bool {
        !results.isEmpty || toolbarInfo.text != nil
    }

    func hideApp() {
        isVisible = false
        query = ""
        results = []
        selectedResultId = nil
        NSApplication.shared.hide(nil)
    }

    func showApp() {
        isVisible = true
        NSApplication.shared.activate(ignoringOtherApps: true)
        if let window = NSApplication.shared.windows.first {
            window.makeKeyAndOrderFront(nil)
        }
    }

    private func onResultSelected() {
        guard let result = selectedResult else {
            isShowPreviewPanel = false
            currentPreview = nil
            updateToolbarWithActions(actions: [])
            actionListController.updateItems([])
            return
        }

        if let preview = result.preview, !preview.previewData.isEmpty {
            currentPreview = preview
            isShowPreviewPanel = true
        } else {
            currentPreview = nil
            isShowPreviewPanel = false
        }

        let actions = result.actions ?? []
        updateToolbarWithActions(actions: actions)
        
        let actionItems = actions.map { WoxListItem.fromResultAction($0) }
        actionListController.updateItems(actionItems)
    }

    private func resetActiveResult() {
        if results.isEmpty {
            resultListController.updateItems([])
            resultGridController.updateItems([])
            return
        }

        if let firstIndex = results.firstIndex(where: { !$0.isGroup }) {
            activeResultIndex = firstIndex
            selectedResultId = results[firstIndex].id
            
            resultListController.updateActiveIndex(firstIndex)
            resultGridController.updateActiveIndex(firstIndex)
        }
    }

    func moveSelection(offset: Int) {
        if isShowActionPanel {
            actionListController.updateActiveIndexByDirection(offset > 0 ? Direction.down : Direction.up)
            selectedActionId = actionListController.items[actionListController.activeIndex].id
        } else {
            if isGridLayout {
                resultGridController.updateActiveIndexByDirection(offset > 0 ? Direction.down : Direction.up)
                activeResultIndex = resultGridController.activeIndex
            } else {
                resultListController.updateActiveIndexByDirection(offset > 0 ? Direction.down : Direction.up)
                activeResultIndex = resultListController.activeIndex
            }
            selectedResultId = results[activeResultIndex].id
        }
    }

    private func updateToolbarWithActions(actions: [WoxResultAction]) {
        var actionsWithHotkeys = actions.filter { !$0.hotkey.isEmpty }
        actionsWithHotkeys.sort { a, b in
            if a.isDefault && !b.isDefault { return false }
            if !a.isDefault && b.isDefault { return true }
            return false
        }

        var toolbarActions = actionsWithHotkeys.map {
            ToolbarActionInfo(name: $0.name, hotkey: $0.hotkey)
        }

        if !actions.isEmpty {
            toolbarActions.append(ToolbarActionInfo(name: "More Actions", hotkey: "cmd+j"))
        }

        toolbarInfo = ToolbarInfo(
            text: toolbarInfo.text,
            icon: toolbarInfo.icon,
            actions: toolbarActions.isEmpty ? nil : toolbarActions
        )
    }

    // MARK: - Refresh Query

    private func refreshQuery(preserveSelectedIndex: Bool) {
        // Save current selection if needed
        if preserveSelectedIndex {
            let activeResults = results.filter { !$0.isGroup }
            if let currentIndex = activeResults.firstIndex(where: { $0.id == selectedResultId }) {
                pendingPreservedIndex = currentIndex
            }
        }

        // Re-send the current query
        if !currentQuery.queryText.isEmpty {
            sendQuery(text: currentQuery.queryText)
        }
    }

    // MARK: - Update Result

    private func updateResult(_ data: [String: Any]) -> Bool {
        guard let resultId = data["Id"] as? String else { return false }

        // Find the result to update
        guard let index = results.firstIndex(where: { $0.id == resultId }) else {
            return false  // Result not found
        }

        var result = results[index]
        var needUpdate = false

        // Update title if provided
        if let title = data["Title"] as? String {
            result = WoxQueryResult(
                id: result.id, queryId: result.queryId, title: title,
                subTitle: result.subTitle, icon: result.icon, score: result.score,
                group: result.group, groupScore: result.groupScore, isGroup: result.isGroup,
                preview: result.preview, tails: result.tails, actions: result.actions
            )
            needUpdate = true
        }

        // Update subtitle if provided
        if let subTitle = data["SubTitle"] as? String {
            result = WoxQueryResult(
                id: result.id, queryId: result.queryId, title: result.title,
                subTitle: subTitle, icon: result.icon, score: result.score,
                group: result.group, groupScore: result.groupScore, isGroup: result.isGroup,
                preview: result.preview, tails: result.tails, actions: result.actions
            )
            needUpdate = true
        }

        // Update icon if provided
        if let iconDict = data["Icon"] as? [String: Any],
            let imageType = iconDict["ImageType"] as? String,
            let imageData = iconDict["ImageData"] as? String
        {
            let newIcon = WoxIcon(imageType: imageType, imageData: imageData)
            result = WoxQueryResult(
                id: result.id, queryId: result.queryId, title: result.title,
                subTitle: result.subTitle, icon: newIcon, score: result.score,
                group: result.group, groupScore: result.groupScore, isGroup: result.isGroup,
                preview: result.preview, tails: result.tails, actions: result.actions
            )
            needUpdate = true
        }

        // Update preview if provided
        if let previewDict = data["Preview"] as? [String: Any],
            let previewType = previewDict["PreviewType"] as? String,
            let previewData = previewDict["PreviewData"] as? String
        {
            let properties = previewDict["PreviewProperties"] as? [String: String] ?? [:]
            let newPreview = WoxPreview(
                previewType: previewType, previewData: previewData, previewProperties: properties)
            result = WoxQueryResult(
                id: result.id, queryId: result.queryId, title: result.title,
                subTitle: result.subTitle, icon: result.icon, score: result.score,
                group: result.group, groupScore: result.groupScore, isGroup: result.isGroup,
                preview: newPreview, tails: result.tails, actions: result.actions
            )
            needUpdate = true

            // Update current preview if this is the selected result
            if result.id == selectedResultId {
                currentPreview = newPreview
                isShowPreviewPanel = !newPreview.previewData.isEmpty
            }
        }

        // Update tails if provided
        if let tailsArray = data["Tails"] as? [[String: Any]] {
            let newTails = tailsArray.compactMap { WoxListItemTail.fromJson($0) }
            result = WoxQueryResult(
                id: result.id, queryId: result.queryId, title: result.title,
                subTitle: result.subTitle, icon: result.icon, score: result.score,
                group: result.group, groupScore: result.groupScore, isGroup: result.isGroup,
                preview: result.preview, tails: newTails, actions: result.actions
            )
            needUpdate = true
        }

        // Update actions if provided
        if let actionsArray = data["Actions"] as? [[String: Any]] {
            let newActions = actionsArray.compactMap { WoxResultAction.fromJson($0) }
            result = WoxQueryResult(
                id: result.id, queryId: result.queryId, title: result.title,
                subTitle: result.subTitle, icon: result.icon, score: result.score,
                group: result.group, groupScore: result.groupScore, isGroup: result.isGroup,
                preview: result.preview, tails: result.tails, actions: newActions
            )
            needUpdate = true

            // Update toolbar if this is the selected result
            if result.id == selectedResultId {
                updateToolbarWithActions(actions: newActions)
            }
        }

        if needUpdate {
            results[index] = result
        }

        return true
    }

    // MARK: - Push Results

    private func pushResults(queryId: String, resultsArray: [[String: Any]]) -> Bool {
        // Only accept results for current query
        if queryId != currentQuery.queryId {
            return false
        }

        var newResults: [WoxQueryResult] = []
        for resultDict in resultsArray {
            if var result = WoxQueryResult.fromJson(resultDict) {
                if result.queryId.isEmpty {
                    result.queryId = queryId
                }
                newResults.append(result)
            }
        }

        if !newResults.isEmpty {
            onReceivedQueryResults(newResults, queryId: queryId)
        }

        return true
    }

    private func handleThemeResponse(_ json: [String: Any]) {
        print("Debug: Received Theme Response")
        if let data = json["Data"] as? [String: Any] {
            print("Debug: Valid Data in Theme Response")
            // print("Debug: Theme Data: \(data)")
            self.theme = WoxTheme.fromJson(data)
            print("Debug: Theme Loaded: \(self.theme.themeName), ActiveBG: \(self.theme.resultItemActiveBackgroundColor)")
        } else {
            print("Error: No Data in Theme Response")
        }
    }

    private func handleMessage(_ json: [String: Any]) {
        let type = json["Type"] as? String
        let method = json["Method"] as? String

        guard let type = type, let method = method else { return }

        if type == "WebsocketMsgTypeResponse" {
            if method == "Query" {
                handleQueryResponse(json)
            } else if method == "Theme" {
                handleThemeResponse(json)
            } else if method == "QueryMRU" {
                handleMRUResponse(json)
            }
        } else if type == "WebsocketMsgTypeRequest" {
            handleServerRequest(method: method, json: json)
        }
    }

    private func handleMRUResponse(_ json: [String: Any]) {
        guard let data = json["Data"] as? [[String: Any]] else { return }

        let queryId = currentQuery.queryId
        var parsedResults: [WoxQueryResult] = []
        for resultDict in data {
            if var result = WoxQueryResult.fromJson(resultDict) {
                result.queryId = queryId
                parsedResults.append(result)
            }
        }

        if !parsedResults.isEmpty {
            onReceivedQueryResults(parsedResults, queryId: queryId)
        }
    }

    // MARK: - Window Management Actions
    var windowActionShow: (() -> Void)?
    var windowActionHide: (() -> Void)?
    var windowActionToggle: (() -> Void)?
    var windowActionSetSize: ((CGFloat, CGFloat) -> Void)?
    var windowActionSetPosition: ((CGFloat, CGFloat) -> Void)?
    var windowActionCenter: ((CGFloat?, CGFloat?) -> Void)?
    var windowActionSetAlwaysOnTop: ((Bool) -> Void)?

    private func handleServerRequest(method: String, json: [String: Any]) {
        switch method {
        case "ShowApp":
            windowActionShow?()
            sendResponse(to: json, success: true, data: nil)
        case "HideApp":
            windowActionHide?()
            sendResponse(to: json, success: true, data: nil)
        case "ChangeQuery":
            if let data = json["Data"] as? [String: Any],
                let queryText = data["queryText"] as? String
            {
                query = queryText
            }
            sendResponse(to: json, success: true, data: nil)
        case "ToggleApp":
            windowActionToggle?()
            sendResponse(to: json, success: true, data: nil)
        case "SetToolbar":
            if let data = json["Data"] as? [String: Any] {
                let text = data["text"] as? String
                let icon = data["icon"] as? String
                toolbarInfo = ToolbarInfo(text: text, icon: icon, actions: toolbarInfo.actions)
            }
            sendResponse(to: json, success: true, data: nil)
        case "RefreshQuery":
            // Refresh the current query with new results
            let preserveSelectedIndex =
                (json["Data"] as? [String: Any])?["preserveSelectedIndex"] as? Bool ?? false
            refreshQuery(preserveSelectedIndex: preserveSelectedIndex)
            sendResponse(to: json, success: true, data: nil)
        case "ChangeTheme":
            if let data = json["Data"] as? [String: Any] {
                theme = WoxTheme.fromJson(data)
            }
            sendResponse(to: json, success: true, data: nil)
        case "GetCurrentQuery":
            let queryData: [String: Any] = [
                "queryId": currentQuery.queryId,
                "queryType": currentQuery.queryType,
                "queryText": currentQuery.queryText,
                "querySelection": [
                    "type": currentQuery.querySelection.type,
                    "text": currentQuery.querySelection.text,
                    "filePaths": currentQuery.querySelection.filePaths,
                ],
            ]
            sendResponse(to: json, success: true, data: queryData)
        case "UpdateResult":
            if let data = json["Data"] as? [String: Any] {
                let success = updateResult(data)
                sendResponse(to: json, success: true, data: success)
            } else {
                sendResponse(to: json, success: false, data: nil)
            }
        case "PushResults":
            if let data = json["Data"] as? [String: Any],
                let queryId = data["QueryId"] as? String,
                let resultsArray = data["Results"] as? [[String: Any]]
            {
                let success = pushResults(queryId: queryId, resultsArray: resultsArray)
                sendResponse(to: json, success: true, data: success)
            } else {
                sendResponse(to: json, success: false, data: nil)
            }
        case "ShowToolbarMsg":
            if let data = json["Data"] as? [String: Any] {
                let text = data["Text"] as? String
                let iconStr = data["Icon"] as? String
                toolbarInfo = ToolbarInfo(text: text, icon: iconStr, actions: toolbarInfo.actions)
            }
            sendResponse(to: json, success: true, data: nil)
        case "QueryMetadata":
            if let data = json["Data"] as? [String: Any] {
                let metadata = QueryMetadata(json: data)

                // Update query icon
                if metadata.icon.imageType.isEmpty {
                    queryIcon = QueryIconInfo.empty()
                } else {
                    queryIcon = QueryIconInfo(icon: metadata.icon, action: nil)
                }

                // Update grid layout
                isGridLayout = metadata.isGridLayout
                gridLayoutParams = metadata.gridLayoutParams
            }
            sendResponse(to: json, success: true, data: nil)
        case "setSize":
             if let width = json["width"] as? Double, let height = json["height"] as? Double {
                  windowActionSetSize?(CGFloat(width), CGFloat(height))
             }
             if let data = json["Data"] as? [String: Any],
                let width = data["width"] as? Double,
                let height = data["height"] as? Double {
                 windowActionSetSize?(CGFloat(width), CGFloat(height))
             }
             sendResponse(to: json, success: true, data: nil)
        case "setPosition":
            if let data = json["Data"] as? [String: Any],
               let x = data["x"] as? Double,
               let y = data["y"] as? Double {
                windowActionSetPosition?(CGFloat(x), CGFloat(y))
            }
            sendResponse(to: json, success: true, data: nil)
        case "center":
            var width: CGFloat?
            var height: CGFloat?
            if let data = json["Data"] as? [String: Any] {
                 if let w = data["width"] as? Double { width = CGFloat(w) }
                 if let h = data["height"] as? Double { height = CGFloat(h) }
            }
            windowActionCenter?(width, height)
            sendResponse(to: json, success: true, data: nil)
        case "setAlwaysOnTop":
            if let alwaysOnTop = (json["Data"] as? Bool) ?? (json["Data"] as? [String: Any])?["alwaysOnTop"] as? Bool {
                windowActionSetAlwaysOnTop?(alwaysOnTop)
            } else if let data = json["Data"] as? [String: Any], let always = data["value"] as? Bool {
                 windowActionSetAlwaysOnTop?(always)
            }
             if let always = json["Data"] as? Bool {
                 windowActionSetAlwaysOnTop?(always)
             }
            sendResponse(to: json, success: true, data: nil)
            
        case "isVisible":
            sendResponse(to: json, success: true, data: isVisible)
            
        case "waitUntilReadyToShow":
             sendResponse(to: json, success: true, data: nil)

        default:
            sendResponse(to: json, success: true, data: nil)
        }
    }

    func toolbarSnooze(text: String, duration: String) {
        let msg = WoxWebsocketMsg(
            method: "toolbarSnooze",
            data: ["text": text, "duration": duration] as [String: Any]
        )
        webSocketManager.send(message: msg)
        
        toolbarInfo = ToolbarInfo(text: nil, icon: nil, actions: toolbarInfo.actions)
    }

    private func sendResponse(to request: [String: Any], success: Bool, data: Any?) {
        guard let requestId = request["RequestId"] as? String,
            let traceId = request["TraceId"] as? String,
            let method = request["Method"] as? String
        else { return }

        let response = WoxWebsocketMsg(
            requestId: requestId,
            traceId: traceId,
            method: method,
            type: .response,
            data: data ?? [:] as [String: Any],
            success: success
        )
        webSocketManager.send(message: response)
    }

    // MARK: - Quick Select Mode

    /// Start the quick select timer when Cmd key is pressed
    func startQuickSelectTimer() {
        // Don't start if already in quick select mode or timer is running
        guard !isQuickSelectMode, quickSelectTimer == nil else { return }

        quickSelectTimer = Timer.scheduledTimer(withTimeInterval: quickSelectDelay, repeats: false)
        { [weak self] _ in
            self?.activateQuickSelectMode()
        }
    }

    /// Stop the quick select timer when Cmd key is released
    func stopQuickSelectTimer() {
        quickSelectTimer?.invalidate()
        quickSelectTimer = nil

        if isQuickSelectMode {
            deactivateQuickSelectMode()
        }
    }

    /// Activate quick select mode - show number labels
    private func activateQuickSelectMode() {
        isQuickSelectMode = true
    }

    /// Deactivate quick select mode - hide number labels
    private func deactivateQuickSelectMode() {
        isQuickSelectMode = false
    }

    /// Handle number key press in quick select mode
    /// Returns true if the key was handled
    func handleQuickSelectNumber(_ number: Int) -> Bool {
        guard isQuickSelectMode else { return false }

        let visibleResults = results.filter { !$0.isGroup }
        let targetIndex = number - 1  // 1-based to 0-based

        guard targetIndex >= 0, targetIndex < visibleResults.count else { return false }

        let result = visibleResults[targetIndex]
        selectedResultId = result.id
        
        let fullIndex = results.firstIndex(where: { $0.id == result.id }) ?? 0
        resultListController.updateActiveIndex(fullIndex)
        resultGridController.updateActiveIndex(fullIndex)
        
        executeAction(result: result)

        return true
    }

    /// Get visible (non-group) results for quick select
    func getVisibleResults() -> [WoxQueryResult] {
        return results.filter { !$0.isGroup }.prefix(9).map { $0 }
    }

    // MARK: - Loading Animation

    /// Start loading timer when query is sent
    private func startLoadingTimer() {
        loadingTimer?.invalidate()
        loadingTimer = Timer.scheduledTimer(withTimeInterval: loadingDelay, repeats: false) {
            [weak self] _ in
            DispatchQueue.main.async {
                // Only show loading if we still don't have results
                if self?.results.isEmpty == true {
                    self?.isLoading = true
                }
            }
        }
    }

    /// Stop loading timer and hide loading indicator
    private func stopLoading() {
        loadingTimer?.invalidate()
        loadingTimer = nil
        isLoading = false
    }
}
