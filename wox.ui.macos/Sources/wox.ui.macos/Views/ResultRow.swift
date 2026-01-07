import SwiftUI
import AppKit

struct ResultRow: View {
    let result: WoxQueryResult
    let isSelected: Bool
    let theme: WoxTheme

    var body: some View {
        if result.isGroup {
            GroupHeader(title: result.title, theme: theme)
        } else {
            HStack(alignment: .center, spacing: 0) {
                // Border Left
                if isSelected && theme.resultItemActiveBorderLeftWidth > 0 {
                    Rectangle()
                        .fill(Color(hex: theme.resultItemActiveBackgroundColor))
                        .frame(width: theme.resultItemActiveBorderLeftWidth)
                } else if theme.resultItemBorderLeftWidth > 0 {
                    Rectangle()
                        .fill(Color(hex: theme.resultItemTitleColor))
                        .frame(width: theme.resultItemBorderLeftWidth)
                }

                // Icon - 30x30 with Flutter-style padding
                WoxIconView(icon: result.icon, size: 30)
                    .padding(.leading, 5)
                    .padding(.trailing, 10)

                VStack(alignment: .leading, spacing: 2) {
                    Text(result.title)
                        .font(.system(size: 16, weight: .medium))
                        .foregroundColor(isSelected ? Color(hex: theme.resultItemActiveTitleColor) : Color(hex: theme.resultItemTitleColor))
                        .lineLimit(1)

                    if let subTitle = result.subTitle, !subTitle.isEmpty {
                        Text(subTitle)
                            .font(.system(size: 13))
                            .foregroundColor(isSelected ? Color(hex: theme.resultItemActiveSubTitleColor) : Color(hex: theme.resultItemSubTitleColor))
                            .lineLimit(1)
                    }
                }
                
                Spacer()

                if let tails = result.tails {
                    HStack(spacing: 10) {
                        ForEach(tails) { tail in
                            HStack(spacing: 4) {
                                if let icon = tail.icon {
                                    WoxIconView(icon: icon, size: 12)
                                }
                                if let text = tail.text {
                                    Text(text)
                                        .font(.system(size: 12))
                                        .foregroundColor(isSelected ? Color(hex: theme.resultItemActiveTailTextColor) : Color(hex: theme.resultItemTailTextColor))
                                }
                            }
                        }
                    }
                    .padding(.trailing, 5)
                }
            }
            .padding(.leading, theme.resultItemPaddingLeft)
            .padding(.trailing, theme.resultItemPaddingRight)
            .padding(.top, theme.resultItemPaddingTop)
            .padding(.bottom, theme.resultItemPaddingBottom)
            .background(
                RoundedRectangle(cornerRadius: theme.resultItemBorderRadius)
                    .fill(isSelected ? Color(hex: theme.resultItemActiveBackgroundColor) : Color.clear)
            )
        }
    }
}

struct GroupHeader: View {
    let title: String
    let theme: WoxTheme
    
    var body: some View {
        HStack {
            Text(title)
                .font(.system(size: 12, weight: .bold))
                .foregroundColor(Color(hex: theme.resultItemSubTitleColor))
            Spacer()
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
        .background(Color.white.opacity(0.02))
    }
}

// MARK: - WoxIconView

struct WoxIconView: View {
    let icon: WoxIcon?
    let size: CGFloat
    
    var body: some View {
        if let icon = icon {
            iconContent(for: icon)
                .frame(width: size, height: size)
        } else {
            Image(systemName: "app.fill")
                .resizable()
                .aspectRatio(contentMode: .fit)
                .foregroundColor(.secondary)
                .frame(width: size, height: size)
        }
    }
    
    @ViewBuilder
    private func iconContent(for icon: WoxIcon) -> some View {
        switch icon.imageType {
        case "absolute":
            if let image = NSImage(contentsOfFile: icon.imageData) {
                Image(nsImage: image)
                    .resizable()
                    .aspectRatio(contentMode: .fit)
            } else {
                let fileIcon = NSWorkspace.shared.icon(forFile: icon.imageData)
                Image(nsImage: fileIcon)
                    .resizable()
                    .aspectRatio(contentMode: .fit)
            }
            
        case "base64":
            if let imageData = parseBase64Image(icon.imageData),
               let nsImage = NSImage(data: imageData) {
                Image(nsImage: nsImage)
                    .resizable()
                    .aspectRatio(contentMode: .fit)
            } else {
                placeholderIcon
            }
            
        case "emoji":
            Text(icon.imageData)
                .font(.system(size: size * 0.8))
            
        case "svg":
            placeholderIcon
            
        case "url":
            AsyncImage(url: URL(string: icon.imageData)) { image in
                image
                    .resizable()
                    .aspectRatio(contentMode: .fit)
            } placeholder: {
                ProgressView()
            }
            
        default:
            placeholderIcon
        }
    }
    
    private var placeholderIcon: some View {
        Image(systemName: "app.fill")
            .resizable()
            .aspectRatio(contentMode: .fit)
            .foregroundColor(.secondary)
    }
    
    private func parseBase64Image(_ data: String) -> Data? {
        if data.contains(";base64,") {
            let parts = data.components(separatedBy: ";base64,")
            if parts.count == 2 {
                return Data(base64Encoded: parts[1])
            }
        }
        return Data(base64Encoded: data)
    }
}
