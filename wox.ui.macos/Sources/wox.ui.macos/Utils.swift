import SwiftUI

extension Color {
    /// Initialize Color from CSS color string
    /// Supports: hex (#RGB, #RRGGBB, #AARRGGBB), rgb(), rgba()
    init(hex: String) {
        let trimmed = hex.trimmingCharacters(in: .whitespacesAndNewlines)
        // print("Debug Color: '\(hex)' -> '\(trimmed)'")
        
        // Try parsing as rgba() format: rgba(r, g, b, a)
        if trimmed.lowercased().hasPrefix("rgba(") && trimmed.hasSuffix(")") {
            let inner = String(trimmed.dropFirst(5).dropLast(1))
            let components = inner.split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }
            if components.count == 4,
               let r = Double(components[0]),
               let g = Double(components[1]),
               let b = Double(components[2]),
               let a = Double(components[3]) {
                self.init(
                    .sRGB,
                    red: r / 255.0,
                    green: g / 255.0,
                    blue: b / 255.0,
                    opacity: a
                )
                return
            }
        }
        
        // Try parsing as rgb() format: rgb(r, g, b)
        if trimmed.lowercased().hasPrefix("rgb(") && trimmed.hasSuffix(")") {
            let inner = String(trimmed.dropFirst(4).dropLast(1))
            let components = inner.split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }
            if components.count == 3,
               let r = Double(components[0]),
               let g = Double(components[1]),
               let b = Double(components[2]) {
                self.init(
                    .sRGB,
                    red: r / 255.0,
                    green: g / 255.0,
                    blue: b / 255.0,
                    opacity: 1.0
                )
                return
            }
        }
        
        // Parse as hex format
        let hex = trimmed.trimmingCharacters(in: CharacterSet.alphanumerics.inverted)
        var int: UInt64 = 0
        Scanner(string: hex).scanHexInt64(&int)
        let a, r, g, b: UInt64
        switch hex.count {
        case 3: // RGB (12-bit)
            (a, r, g, b) = (255, (int >> 8) * 17, (int >> 4 & 0xF) * 17, (int & 0xF) * 17)
        case 6: // RGB (24-bit)
            (a, r, g, b) = (255, int >> 16, int >> 8 & 0xFF, int & 0xFF)
        case 8: // ARGB (32-bit)
            (a, r, g, b) = (int >> 24, int >> 16 & 0xFF, int >> 8 & 0xFF, int & 0xFF)
        default:
            (a, r, g, b) = (255, 0, 0, 0)
        }

        self.init(
            .sRGB,
            red: Double(r) / 255,
            green: Double(g) / 255,
            blue: Double(b) / 255,
            opacity: Double(a) / 255
        )
    }
}
