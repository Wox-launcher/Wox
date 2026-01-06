import SwiftUI
import AppKit

struct ResultRow: View {
    let result: WoxQueryResult
    let isSelected: Bool

    var body: some View {
        HStack(alignment: .center, spacing: 12) {
            // Icon
            WoxIconView(icon: result.icon, size: 32)

            VStack(alignment: .leading, spacing: 2) {
                Text(result.title)
                    .font(.system(size: 16, weight: .medium))
                    .foregroundColor(isSelected ? .white : .primary)
                    .lineLimit(1)

                if let subTitle = result.subTitle, !subTitle.isEmpty {
                    Text(subTitle)
                        .font(.system(size: 13))
                        .foregroundColor(isSelected ? .white.opacity(0.8) : .secondary)
                        .lineLimit(1)
                }
            }
            
            Spacer()

            if let tails = result.tails {
                HStack(spacing: 6) {
                    ForEach(tails) { tail in
                        HStack(spacing: 4) {
                            if let icon = tail.icon {
                                WoxIconView(icon: icon, size: 12)
                            }
                            if let text = tail.text {
                                Text(text)
                                    .font(.system(size: 11, weight: .medium, design: .monospaced))
                            }
                        }
                        .padding(.horizontal, 6)
                        .padding(.vertical, 2)
                        .background(Color.white.opacity(isSelected ? 0.2 : 0.05))
                        .cornerRadius(4)
                        .foregroundColor(isSelected ? .white.opacity(0.9) : .secondary)
                    }
                }
            }

            if isSelected {
                Image(systemName: "return")
                    .font(.system(size: 12, weight: .bold))
                    .foregroundColor(.white.opacity(0.6))
            }
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
        .background(isSelected ? Color.accentColor : Color.clear)
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
            // File path
            if let image = NSImage(contentsOfFile: icon.imageData) {
                Image(nsImage: image)
                    .resizable()
                    .aspectRatio(contentMode: .fit)
            } else {
                // Try to get file icon
                let fileIcon = NSWorkspace.shared.icon(forFile: icon.imageData)
                Image(nsImage: fileIcon)
                    .resizable()
                    .aspectRatio(contentMode: .fit)
            }
            
        case "base64":
            // Base64 encoded image
            if let imageData = parseBase64Image(icon.imageData),
               let nsImage = NSImage(data: imageData) {
                Image(nsImage: nsImage)
                    .resizable()
                    .aspectRatio(contentMode: .fit)
            } else {
                placeholderIcon
            }
            
        case "emoji":
            // Emoji
            Text(icon.imageData)
                .font(.system(size: size * 0.8))
            
        case "svg":
            // SVG - for now show placeholder, SVG requires additional handling
            // Could use WebView or convert to NSImage
            placeholderIcon
            
        case "url":
            // URL image
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
        // Format: "image/png;base64,xxxxx" or just base64 data
        if data.contains(";base64,") {
            let parts = data.components(separatedBy: ";base64,")
            if parts.count == 2 {
                return Data(base64Encoded: parts[1])
            }
        }
        return Data(base64Encoded: data)
    }
}
