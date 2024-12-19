class WoxLang {
  final String code;
  final String name;

  WoxLang({required this.code, required this.name});

  factory WoxLang.fromJson(Map<String, dynamic> json) {
    return WoxLang(
      code: json['Code'],
      name: json['Name'],
    );
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Code'] = code;
    data['Name'] = name;
    return data;
  }
}
