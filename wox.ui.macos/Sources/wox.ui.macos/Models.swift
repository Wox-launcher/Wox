import Foundation

// MARK: - Constants

let MAX_LIST_VIEW_ITEM_COUNT = 10
let QUERY_BOX_BASE_HEIGHT: CGFloat = 55.0
let QUERY_BOX_CONTENT_PADDING_TOP: CGFloat = 4.0
let QUERY_BOX_CONTENT_PADDING_BOTTOM: CGFloat = 17.0
let QUERY_BOX_LINE_HEIGHT: CGFloat = QUERY_BOX_BASE_HEIGHT - QUERY_BOX_CONTENT_PADDING_TOP - QUERY_BOX_CONTENT_PADDING_BOTTOM
let QUERY_BOX_MAX_LINES = 4
let RESULT_ITEM_BASE_HEIGHT: CGFloat = 50.0
let TOOLBAR_HEIGHT: CGFloat = 40.0

// MARK: - PlainQuery

struct PlainQuery: Equatable {
    var queryId: String
    var queryType: String
    var queryText: String
    var querySelection: Selection
    
    var isEmpty: Bool {
        return queryText.isEmpty && querySelection.type.isEmpty
    }
    
    static func empty() -> PlainQuery {
        return PlainQuery(queryId: "", queryType: "", queryText: "", querySelection: Selection.empty())
    }
    
    static func text(_ text: String) -> PlainQuery {
        return PlainQuery(queryId: "", queryType: "input", queryText: text, querySelection: Selection.empty())
    }
    
    static func emptyInput() -> PlainQuery {
        return PlainQuery(queryId: "", queryType: "input", queryText: "", querySelection: Selection.empty())
    }
}

struct Selection: Equatable {
    var type: String
    var text: String
    var filePaths: [String]
    
    static func empty() -> Selection {
        return Selection(type: "", text: "", filePaths: [])
    }
}

// MARK: - Query History

struct QueryHistory {
    var query: PlainQuery?
    var timestamp: Int?
    
    init(json: [String: Any]) {
        if let queryJson = json["Query"] as? [String: Any] {
            query = PlainQuery(json: queryJson)
        } else {
            query = nil
        }
        timestamp = json["Timestamp"] as? Int
    }
}

// MARK: - Query Result

struct WoxQueryResult: Identifiable, Equatable {
    let id: String
    var queryId: String
    let title: String
    let subTitle: String?
    let icon: WoxIcon?
    var score: Int
    let group: String
    let groupScore: Int
    var isGroup: Bool
    var preview: WoxPreview?
    var tails: [WoxListItemTail]?
    var actions: [WoxResultAction]?
    
    static func empty() -> WoxQueryResult {
        return WoxQueryResult(
            id: "",
            queryId: "",
            title: "",
            subTitle: "",
            icon: nil,
            score: 0,
            group: "",
            groupScore: 0,
            isGroup: false,
            preview: nil,
            tails: [],
            actions: []
        )
    }
    
    static func fromJson(_ dict: [String: Any]) -> WoxQueryResult? {
        guard let id = dict["Id"] as? String,
              let title = dict["Title"] as? String else {
            return nil
        }
        
        let subTitle = dict["SubTitle"] as? String
        let score = dict["Score"] as? Int ?? 0
        let group = dict["Group"] as? String ?? ""
        let groupScore = dict["GroupScore"] as? Int ?? 0
        let queryId = dict["QueryId"] as? String ?? ""
        
        var icon: WoxIcon? = nil
        if let iconDict = dict["Icon"] as? [String: Any],
           let imageType = iconDict["ImageType"] as? String,
           let imageData = iconDict["ImageData"] as? String {
            icon = WoxIcon(imageType: imageType, imageData: imageData)
        }
        
        var actions: [WoxResultAction]? = nil
        if let actionsArray = dict["Actions"] as? [[String: Any]] {
            actions = actionsArray.compactMap { WoxResultAction.fromJson($0) }
        }
        
        var preview: WoxPreview? = nil
        if let previewDict = dict["Preview"] as? [String: Any],
           let previewType = previewDict["PreviewType"] as? String,
           let previewData = previewDict["PreviewData"] as? String {
            let properties = previewDict["PreviewProperties"] as? [String: String] ?? [:]
            preview = WoxPreview(previewType: previewType, previewData: previewData, previewProperties: properties)
        }
        
        var tails: [WoxListItemTail]? = nil
        if let tailsArray = dict["Tails"] as? [[String: Any]] {
            tails = tailsArray.compactMap { WoxListItemTail.fromJson($0) }
        }
        
        return WoxQueryResult(
            id: id,
            queryId: queryId,
            title: title,
            subTitle: subTitle,
            icon: icon,
            score: score,
            group: group,
            groupScore: groupScore,
            isGroup: false,
            preview: preview,
            tails: tails,
            actions: actions
        )
    }
}

// MARK: - Result Action

struct WoxResultAction: Identifiable, Equatable {
    let id: String
    var type: String
    let name: String
    let icon: WoxIcon?
    var isDefault: Bool
    var preventHideAfterAction: Bool
    var hotkey: String
    var isSystemAction: Bool
    var resultId: String
    var contextData: [String: String]
    var form: [PluginSettingDefinitionItem]?
    
    static func empty() -> WoxResultAction {
        return WoxResultAction(
            id: "",
            type: "execute",
            name: "",
            icon: nil,
            isDefault: false,
            preventHideAfterAction: false,
            hotkey: "",
            isSystemAction: false,
            resultId: "",
            contextData: [:],
            form: []
        )
    }
    
    static func fromJson(_ dict: [String: Any]) -> WoxResultAction? {
        guard let id = dict["Id"] as? String,
              let name = dict["Name"] as? String else {
            return nil
        }
        
        let type = dict["Type"] as? String ?? "execute"
        let isDefault = dict["IsDefault"] as? Bool ?? false
        let preventHideAfterAction = dict["PreventHideAfterAction"] as? Bool ?? false
        let hotkey = dict["Hotkey"] as? String ?? ""
        let isSystemAction = dict["IsSystemAction"] as? Bool ?? false
        let resultId = dict["ResultId"] as? String ?? ""
        
        var icon: WoxIcon? = nil
        if let iconDict = dict["Icon"] as? [String: Any],
           let imageType = iconDict["ImageType"] as? String,
           let imageData = iconDict["ImageData"] as? String {
            icon = WoxIcon(imageType: imageType, imageData: imageData)
        }
        
        var contextData: [String: String] = [:]
        if let rawContextData = dict["ContextData"] {
            if let data = rawContextData as? [String: String] {
                contextData = data
            } else if let dataStr = rawContextData as? String, !dataStr.isEmpty {
                if let jsonData = dataStr.data(using: .utf8),
                   let json = try? JSONSerialization.jsonObject(with: jsonData) as? [String: Any] {
                    contextData = json.compactMapValues { $0 as? String }
                }
            }
        }
        
        var form: [PluginSettingDefinitionItem]? = nil
        if let formArray = dict["Form"] as? [[String: Any]] {
            form = formArray.compactMap { PluginSettingDefinitionItem.fromJson($0) }
        }
        
        return WoxResultAction(
            id: id,
            type: type,
            name: name,
            icon: icon,
            isDefault: isDefault,
            preventHideAfterAction: preventHideAfterAction,
            hotkey: hotkey,
            isSystemAction: isSystemAction,
            resultId: resultId,
            contextData: contextData,
            form: form
        )
    }
}

// MARK: - Plugin Setting Definition Item

struct PluginSettingDefinitionItem: Equatable {
    let type: String
    let value: String
    let disabledInPlatforms: String
    let isPlatformSpecific: Bool
    
    static func fromJson(_ dict: [String: Any]) -> PluginSettingDefinitionItem? {
        guard let type = dict["Type"] as? String else { return nil }
        
        let value = dict["Value"] as? String ?? ""
        let disabledInPlatforms = dict["DisabledInPlatforms"] as? String ?? ""
        let isPlatformSpecific = dict["IsPlatformSpecific"] as? Bool ?? false
        
        return PluginSettingDefinitionItem(
            type: type,
            value: value,
            disabledInPlatforms: disabledInPlatforms,
            isPlatformSpecific: isPlatformSpecific
        )
    }
}

// MARK: - Preview

struct WoxPreview: Equatable {
    let previewType: String
    let previewData: String
    let previewProperties: [String: String]
    
    var isEmpty: Bool {
        return previewData.isEmpty
    }
    
    static func empty() -> WoxPreview {
        return WoxPreview(previewType: "", previewData: "", previewProperties: [:])
    }
}

// MARK: - Tail

struct WoxListItemTail: Identifiable, Equatable {
    let id = UUID()
    let type: String
    let text: String?
    let icon: WoxIcon?
    
    static func fromJson(_ dict: [String: Any]) -> WoxListItemTail? {
        guard let type = dict["Type"] as? String else { return nil }
        
        let text = dict["Text"] as? String
        var icon: WoxIcon? = nil
        if let iconDict = dict["Icon"] as? [String: Any],
           let imageType = iconDict["ImageType"] as? String,
           let imageData = iconDict["ImageData"] as? String {
            icon = WoxIcon(imageType: imageType, imageData: imageData)
        }
        
        return WoxListItemTail(type: type, text: text, icon: icon)
    }
}

// MARK: - WoxIcon

struct WoxIcon: Equatable {
    let imageType: String
    let imageData: String
    
    static func empty() -> WoxIcon {
        return WoxIcon(imageType: "", imageData: "")
    }
}

// MARK: - WoxTheme

struct WoxTheme {
    var themeId: String
    var themeName: String
    var themeAuthor: String
    var themeUrl: String
    var version: String
    var description: String
    var isSystem: Bool
    var isInstalled: Bool
    var isUpgradable: Bool
    var isAutoAppearance: Bool
    var darkThemeId: String
    var lightThemeId: String
    
    var appBackgroundColor: String
    var resultItemTitleColor: String
    var resultItemSubTitleColor: String
    var resultItemTailTextColor: String
    var resultItemActiveBackgroundColor: String
    var resultItemActiveTitleColor: String
    var resultItemActiveSubTitleColor: String
    var resultItemActiveTailTextColor: String
    var queryBoxFontColor: String
    var queryBoxBackgroundColor: String
    var queryBoxCursorColor: String
    var queryBoxTextSelectionBackgroundColor: String
    var queryBoxTextSelectionColor: String
    var actionContainerBackgroundColor: String
    var actionContainerHeaderFontColor: String
    var actionItemActiveBackgroundColor: String
    var actionItemActiveFontColor: String
    var actionItemFontColor: String
    var actionQueryBoxFontColor: String
    var actionQueryBoxBackgroundColor: String
    var previewFontColor: String
    var previewSplitLineColor: String
    var previewPropertyTitleColor: String
    var previewPropertyContentColor: String
    var previewTextSelectionColor: String
    var toolbarFontColor: String
    var toolbarBackgroundColor: String
    
    var appPaddingLeft: CGFloat
    var appPaddingTop: CGFloat
    var appPaddingRight: CGFloat
    var appPaddingBottom: CGFloat
    var resultContainerPaddingLeft: CGFloat
    var resultContainerPaddingTop: CGFloat
    var resultContainerPaddingRight: CGFloat
    var resultContainerPaddingBottom: CGFloat
    var resultItemPaddingLeft: CGFloat
    var resultItemPaddingTop: CGFloat
    var resultItemPaddingRight: CGFloat
    var resultItemPaddingBottom: CGFloat
    var actionContainerPaddingLeft: CGFloat
    var actionContainerPaddingTop: CGFloat
    var actionContainerPaddingRight: CGFloat
    var actionContainerPaddingBottom: CGFloat
    var toolbarPaddingLeft: CGFloat
    var toolbarPaddingRight: CGFloat
    
    var resultItemBorderRadius: CGFloat
    var queryBoxBorderRadius: CGFloat
    var actionQueryBoxBorderRadius: CGFloat
    
    var resultItemBorderLeftWidth: CGFloat
    var resultItemActiveBorderLeftWidth: CGFloat
    
    static func empty() -> WoxTheme {
        return WoxTheme(
            themeId: "",
            themeName: "",
            themeAuthor: "",
            themeUrl: "",
            version: "",
            description: "",
            isSystem: false,
            isInstalled: false,
            isUpgradable: false,
            isAutoAppearance: false,
            darkThemeId: "",
            lightThemeId: "",
            appBackgroundColor: "#000000",
            resultItemTitleColor: "#FFFFFF",
            resultItemSubTitleColor: "#999999",
            resultItemTailTextColor: "#666666",
            resultItemActiveBackgroundColor: "#007AFF",
            resultItemActiveTitleColor: "#FFFFFF",
            resultItemActiveSubTitleColor: "#FFFFFF",
            resultItemActiveTailTextColor: "#FFFFFF",
            queryBoxFontColor: "#FFFFFF",
            queryBoxBackgroundColor: "#1E1E1E",
            queryBoxCursorColor: "#007AFF",
            queryBoxTextSelectionBackgroundColor: "#007AFF",
            queryBoxTextSelectionColor: "#FFFFFF",
            actionContainerBackgroundColor: "#2C2C2C",
            actionContainerHeaderFontColor: "#999999",
            actionItemActiveBackgroundColor: "#007AFF",
            actionItemActiveFontColor: "#FFFFFF",
            actionItemFontColor: "#FFFFFF",
            actionQueryBoxFontColor: "#FFFFFF",
            actionQueryBoxBackgroundColor: "#1E1E1E",
            previewFontColor: "#FFFFFF",
            previewSplitLineColor: "#333333",
            previewPropertyTitleColor: "#999999",
            previewPropertyContentColor: "#CCCCCC",
            previewTextSelectionColor: "#007AFF",
            toolbarFontColor: "#999999",
            toolbarBackgroundColor: "#1E1E1E",
            appPaddingLeft: 0,
            appPaddingTop: 0,
            appPaddingRight: 0,
            appPaddingBottom: 0,
            resultContainerPaddingLeft: 0,
            resultContainerPaddingTop: 0,
            resultContainerPaddingRight: 0,
            resultContainerPaddingBottom: 0,
            resultItemPaddingLeft: 16,
            resultItemPaddingTop: 8,
            resultItemPaddingRight: 16,
            resultItemPaddingBottom: 8,
            actionContainerPaddingLeft: 12,
            actionContainerPaddingTop: 12,
            actionContainerPaddingRight: 12,
            actionContainerPaddingBottom: 12,
            toolbarPaddingLeft: 16,
            toolbarPaddingRight: 16,
            resultItemBorderRadius: 8,
            queryBoxBorderRadius: 8,
            actionQueryBoxBorderRadius: 6,
            resultItemBorderLeftWidth: 0,
            resultItemActiveBorderLeftWidth: 0
        )
    }
    
    static func fromJson(_ dict: [String: Any]) -> WoxTheme {
        return WoxTheme(
            themeId: dict["ThemeId"] as? String ?? "",
            themeName: dict["ThemeName"] as? String ?? "",
            themeAuthor: dict["ThemeAuthor"] as? String ?? "",
            themeUrl: dict["ThemeUrl"] as? String ?? "",
            version: dict["Version"] as? String ?? "",
            description: dict["Description"] as? String ?? "",
            isSystem: dict["IsSystem"] as? Bool ?? false,
            isInstalled: dict["IsInstalled"] as? Bool ?? false,
            isUpgradable: dict["IsUpgradable"] as? Bool ?? false,
            isAutoAppearance: dict["IsAutoAppearance"] as? Bool ?? false,
            darkThemeId: dict["DarkThemeId"] as? String ?? "",
            lightThemeId: dict["LightThemeId"] as? String ?? "",
            appBackgroundColor: dict["AppBackgroundColor"] as? String ?? "#000000",
            resultItemTitleColor: dict["ResultItemTitleColor"] as? String ?? "#FFFFFF",
            resultItemSubTitleColor: dict["ResultItemSubTitleColor"] as? String ?? "#999999",
            resultItemTailTextColor: dict["ResultItemTailTextColor"] as? String ?? "#666666",
            resultItemActiveBackgroundColor: dict["ResultItemActiveBackgroundColor"] as? String ?? "#007AFF",
            resultItemActiveTitleColor: dict["ResultItemActiveTitleColor"] as? String ?? "#FFFFFF",
            resultItemActiveSubTitleColor: dict["ResultItemActiveSubTitleColor"] as? String ?? "#FFFFFF",
            resultItemActiveTailTextColor: dict["ResultItemActiveTailTextColor"] as? String ?? "#FFFFFF",
            queryBoxFontColor: dict["QueryBoxFontColor"] as? String ?? "#FFFFFF",
            queryBoxBackgroundColor: dict["QueryBoxBackgroundColor"] as? String ?? "#1E1E1E",
            queryBoxCursorColor: dict["QueryBoxCursorColor"] as? String ?? "#007AFF",
            queryBoxTextSelectionBackgroundColor: dict["QueryBoxTextSelectionBackgroundColor"] as? String ?? dict["QueryBoxTextSelectionColor"] as? String ?? "#007AFF",
            queryBoxTextSelectionColor: dict["QueryBoxTextSelectionColor"] as? String ?? dict["ResultItemActiveTitleColor"] as? String ?? "#FFFFFF",
            actionContainerBackgroundColor: dict["ActionContainerBackgroundColor"] as? String ?? "#2C2C2C",
            actionContainerHeaderFontColor: dict["ActionContainerHeaderFontColor"] as? String ?? "#999999",
            actionItemActiveBackgroundColor: dict["ActionItemActiveBackgroundColor"] as? String ?? "#007AFF",
            actionItemActiveFontColor: dict["ActionItemActiveFontColor"] as? String ?? "#FFFFFF",
            actionItemFontColor: dict["ActionItemFontColor"] as? String ?? "#FFFFFF",
            actionQueryBoxFontColor: dict["ActionQueryBoxFontColor"] as? String ?? "#FFFFFF",
            actionQueryBoxBackgroundColor: dict["ActionQueryBoxBackgroundColor"] as? String ?? "#1E1E1E",
            previewFontColor: dict["PreviewFontColor"] as? String ?? "#FFFFFF",
            previewSplitLineColor: dict["PreviewSplitLineColor"] as? String ?? "#333333",
            previewPropertyTitleColor: dict["PreviewPropertyTitleColor"] as? String ?? "#999999",
            previewPropertyContentColor: dict["PreviewPropertyContentColor"] as? String ?? "#CCCCCC",
            previewTextSelectionColor: dict["PreviewTextSelectionColor"] as? String ?? "#007AFF",
            toolbarFontColor: dict["ToolbarFontColor"] as? String ?? "#999999",
            toolbarBackgroundColor: dict["ToolbarBackgroundColor"] as? String ?? "#1E1E1E",
            appPaddingLeft: CGFloat(dict["AppPaddingLeft"] as? Int ?? 0),
            appPaddingTop: CGFloat(dict["AppPaddingTop"] as? Int ?? 0),
            appPaddingRight: CGFloat(dict["AppPaddingRight"] as? Int ?? 0),
            appPaddingBottom: CGFloat(dict["AppPaddingBottom"] as? Int ?? 0),
            resultContainerPaddingLeft: CGFloat(dict["ResultContainerPaddingLeft"] as? Int ?? 0),
            resultContainerPaddingTop: CGFloat(dict["ResultContainerPaddingTop"] as? Int ?? 0),
            resultContainerPaddingRight: CGFloat(dict["ResultContainerPaddingRight"] as? Int ?? 0),
            resultContainerPaddingBottom: CGFloat(dict["ResultContainerPaddingBottom"] as? Int ?? 0),
            resultItemPaddingLeft: CGFloat(dict["ResultItemPaddingLeft"] as? Int ?? 16),
            resultItemPaddingTop: CGFloat(dict["ResultItemPaddingTop"] as? Int ?? 8),
            resultItemPaddingRight: CGFloat(dict["ResultItemPaddingRight"] as? Int ?? 16),
            resultItemPaddingBottom: CGFloat(dict["ResultItemPaddingBottom"] as? Int ?? 8),
            actionContainerPaddingLeft: CGFloat(dict["ActionContainerPaddingLeft"] as? Int ?? 12),
            actionContainerPaddingTop: CGFloat(dict["ActionContainerPaddingTop"] as? Int ?? 12),
            actionContainerPaddingRight: CGFloat(dict["ActionContainerPaddingRight"] as? Int ?? 12),
            actionContainerPaddingBottom: CGFloat(dict["ActionContainerPaddingBottom"] as? Int ?? 12),
            toolbarPaddingLeft: CGFloat(dict["ToolbarPaddingLeft"] as? Int ?? 16),
            toolbarPaddingRight: CGFloat(dict["ToolbarPaddingRight"] as? Int ?? 16),
            resultItemBorderRadius: CGFloat(dict["ResultItemBorderRadius"] as? Int ?? 8),
            queryBoxBorderRadius: CGFloat(dict["QueryBoxBorderRadius"] as? Int ?? 8),
            actionQueryBoxBorderRadius: CGFloat(dict["ActionQueryBoxBorderRadius"] as? Int ?? 6),
            resultItemBorderLeftWidth: CGFloat(parseBorderWidth(dict["ResultItemBorderLeftWidth"] as? Int, dict["ResultItemBorderLeft"] as? Int, defaultValue: 0)),
            resultItemActiveBorderLeftWidth: CGFloat(parseBorderWidth(dict["ResultItemActiveBorderLeftWidth"] as? Int, dict["ResultItemActiveBorderLeft"] as? Int, defaultValue: 0))
        )
    }
    
    private static func parseBorderWidth(_ width: Int?, _ legacy: Int?, defaultValue: Int) -> Int {
        if let w = width { return w }
        if let l = legacy { return l }
        return defaultValue
    }
}

// MARK: - Toolbar

struct ToolbarInfo {
    var text: String?
    var icon: String?
    var actions: [ToolbarActionInfo]?
}

struct ToolbarActionInfo: Equatable {
    let name: String
    let hotkey: String
}

// MARK: - Position

struct Position {
    var type: String
    var x: Int
    var y: Int
    
    init(json: [String: Any]) {
        type = json["Type"] as? String ?? ""
        x = json["X"] as? Int ?? 0
        y = json["Y"] as? Int ?? 0
    }
}

// MARK: - ShowAppParams

struct ShowAppParams {
    var selectAll: Bool
    var position: Position
    var queryHistories: [QueryHistory]
    var launchMode: String
    var startPage: String
    var isQueryFocus: Bool
    
    init(json: [String: Any]) {
        selectAll = json["SelectAll"] as? Bool ?? false
        position = Position(json: json["Position"] as? [String: Any] ?? [:])
        
        var histories: [QueryHistory] = []
        if let historiesArray = json["QueryHistories"] as? [[String: Any]] {
            histories = historiesArray.map { QueryHistory(json: $0) }
        }
        queryHistories = histories
        
        launchMode = json["LaunchMode"] as? String ?? "continue"
        startPage = json["StartPage"] as? String ?? "mru"
        isQueryFocus = json["IsQueryFocus"] as? Bool ?? false
    }
}

// MARK: - QueryMetadata

struct QueryMetadata {
    var icon: WoxIcon
    var resultPreviewWidthRatio: CGFloat
    var isGridLayout: Bool
    var gridLayoutParams: GridLayoutParams
    
    init(json: [String: Any]) {
        if let iconDict = json["Icon"] as? [String: Any],
           let imageType = iconDict["ImageType"] as? String,
           let imageData = iconDict["ImageData"] as? String {
            icon = WoxIcon(imageType: imageType, imageData: imageData)
        } else {
            icon = WoxIcon(imageType: "", imageData: "")
        }
        
        resultPreviewWidthRatio = CGFloat(json["WidthRatio"] as? Double ?? 0.5)
        isGridLayout = json["IsGridLayout"] as? Bool ?? false
        gridLayoutParams = GridLayoutParams(json: json["GridLayoutParams"] as? [String: Any] ?? [:])
    }
}

// MARK: - GridLayoutParams

struct GridLayoutParams {
    var columns: Int
    var showTitle: Bool
    var itemPadding: CGFloat
    var itemMargin: CGFloat
    var commands: [String]
    
    init(json: [String: Any]) {
        columns = json["Columns"] as? Int ?? 8
        showTitle = json["ShowTitle"] as? Bool ?? false
        itemPadding = CGFloat(json["ItemPadding"] as? Int ?? 12)
        itemMargin = CGFloat(json["ItemMargin"] as? Int ?? 6)
        commands = (json["Commands"] as? [String]) ?? []
    }
    
    static func empty() -> GridLayoutParams {
        return GridLayoutParams(json: [:])
    }
}

// MARK: - PlainQuery Extension

extension PlainQuery {
    init(json: [String: Any]) {
        queryId = json["QueryId"] as? String ?? ""
        queryType = json["QueryType"] as? String ?? ""
        queryText = json["QueryText"] as? String ?? ""
        querySelection = Selection(json: json["QuerySelection"] as? [String: Any] ?? [:])
    }
}

extension Selection {
    init(json: [String: Any]) {
        type = json["Type"] as? String ?? ""
        text = json["Text"] as? String ?? ""
        filePaths = (json["FilePaths"] as? [String]) ?? []
    }
}

// MARK: - Array compactMap

extension Array {
    func compactMap<T>(_ transform: (Element) throws -> T?) rethrows -> [T] {
        var result: [T] = []
        for element in self {
            if let value = try? transform(element) {
                result.append(value)
            }
        }
        return result
    }
}

extension Dictionary {
    func compactMapValues<T>(_ transform: (Value) throws -> T?) rethrows -> [Key: T] {
        var result: [Key: T] = [:]
        for (key, value) in self {
            if let mappedValue = try? transform(value) {
                result[key] = mappedValue
            }
        }
        return result
    }
}
