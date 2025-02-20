class Strings {
  /// Format a string with a list of arguments.
  ///
  /// The format is like `%s`, `%d`, `%.1f`, etc. Golang format.
  static String format(String str, List<dynamic> args) {
    for (var arg in args) {
      if (arg is String) {
        str = str.replaceFirst('%s', arg);
      } else if (arg is int) {
        str = str.replaceFirst('%d', arg.toString());
      } else if (arg is double) {
        str = str.replaceFirst('%f', arg.toStringAsFixed(1));
      }
    }

    return str;
  }
}
