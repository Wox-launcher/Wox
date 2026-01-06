import Foundation

// MARK: - Query Result

struct WoxQueryResult: Identifiable {
    let id: String
    let title: String
    let subTitle: String?
    let icon: WoxIcon?
    let score: Int
    let group: String?
    var actions: [WoxResultAction]?
    var preview: WoxPreview?
    var tails: [WoxListItemTail]?
    let queryId: String?
}

struct WoxResultAction: Identifiable {
    let id: String
    let name: String
    let isDefault: Bool
    let hotkey: String
}

// MARK: - Preview

struct WoxPreview {
    let previewType: String
    let previewData: String
    let previewProperties: [String: String]
}

// MARK: - Toolbar

struct ToolbarInfo {
    let text: String?
    let icon: String?
    let actions: [ToolbarActionInfo]?
}

struct ToolbarActionInfo {
    let name: String
    let hotkey: String
}

// MARK: - Tail

struct WoxListItemTail: Identifiable {
    let id = UUID()
    let type: String
    let text: String?
    let icon: WoxIcon?
}

// MARK: - WoxIcon

struct WoxIcon {
    let imageType: String  // "absolute", "base64", "emoji", "svg", "url", etc.
    let imageData: String
}

// MARK: - Query

struct PlainQuery {
    var queryId: String
    var queryType: String
    var queryText: String
}
