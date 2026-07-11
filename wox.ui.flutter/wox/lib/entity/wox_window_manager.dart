class WindowManagerRect {
  late int x;
  late int y;
  late int width;
  late int height;

  WindowManagerRect({required this.x, required this.y, required this.width, required this.height});

  WindowManagerRect.fromJson(Map<String, dynamic> json) {
    x = json['X'] ?? 0;
    y = json['Y'] ?? 0;
    width = json['Width'] ?? 0;
    height = json['Height'] ?? 0;
  }

  Map<String, dynamic> toJson() {
    return <String, dynamic>{'X': x, 'Y': y, 'Width': width, 'Height': height};
  }
}

class WindowManagerDisplay {
  late String id;
  late WindowManagerRect bounds;
  late WindowManagerRect workArea;
  late bool isPrimary;

  WindowManagerDisplay({required this.id, required this.bounds, required this.workArea, required this.isPrimary});

  WindowManagerDisplay.fromJson(Map<String, dynamic> json) {
    id = json['Id'] ?? '';
    bounds = json['Bounds'] != null ? WindowManagerRect.fromJson(json['Bounds']) : WindowManagerRect(x: 0, y: 0, width: 0, height: 0);
    workArea = json['WorkArea'] != null ? WindowManagerRect.fromJson(json['WorkArea']) : WindowManagerRect(x: 0, y: 0, width: 0, height: 0);
    isPrimary = json['IsPrimary'] ?? false;
  }

  Map<String, dynamic> toJson() {
    return <String, dynamic>{'Id': id, 'Bounds': bounds.toJson(), 'WorkArea': workArea.toJson(), 'IsPrimary': isPrimary};
  }
}
