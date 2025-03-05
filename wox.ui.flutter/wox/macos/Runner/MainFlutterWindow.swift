import Cocoa
import FlutterMacOS

class MainFlutterWindow: NSPanel {
  var isReadyToShow: Bool = false
  
  override func awakeFromNib() {
    let flutterViewController = FlutterViewController()
    let windowFrame = self.frame
    self.contentViewController = flutterViewController
    self.setFrame(windowFrame, display: false)

    RegisterGeneratedPlugins(registry: flutterViewController)

    super.awakeFromNib()
  }

  override public func order(_ place: NSWindow.OrderingMode, relativeTo otherWin: Int) {
    super.order(place, relativeTo: otherWin)
    
    if !isReadyToShow {
      setIsVisible(false)
    }
  }
}