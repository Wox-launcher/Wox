// swift-tools-version: 5.9
// The swift-tools-version declares the minimum version of Swift required to build this package.

import PackageDescription

let package = Package(
    name: "wox.ui.macos",
    platforms: [
        .macOS(.v13)
    ],
    products: [
        .executable(name: "wox.ui.macos", targets: ["wox.ui.macos"]),
    ],
    dependencies: [
        .package(url: "https://github.com/daltoniam/Starscream.git", from: "4.0.0")
    ],
    targets: [
        .executableTarget(
            name: "wox.ui.macos",
            dependencies: ["Starscream"],
            path: "Sources"
        ),
    ]
)
