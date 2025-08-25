typedef WoxPluginRuntime = String;

// Runtime types for plugins
// Keep values uppercase to match server payloads and simple comparisons
enum WoxPluginRuntimeEnum {
  GO("GO"),
  PYTHON("PYTHON"),
  NODEJS("NODEJS"),
  SCRIPT("SCRIPT");

  final String value;

  const WoxPluginRuntimeEnum(this.value);

  // Compare a runtime string with enum (case-insensitive)
  static bool equals(String runtime, WoxPluginRuntimeEnum target) {
    // normalize to uppercase to be robust
    return runtime.toUpperCase() == target.value;
  }
}
