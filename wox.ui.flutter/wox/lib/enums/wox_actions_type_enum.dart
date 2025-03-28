typedef WoxActionsType = String;

enum WoxActionsTypeEnum {
  /// The action type for the result.
  WOX_ACTIONS_TYPE_RESULT("result", "result"),

  /// The action type for select the ai model.
  WOX_ACTIONS_TYPE_AI_MODEL("ai_model", "ai_model");

  final String code;
  final String value;

  const WoxActionsTypeEnum(this.code, this.value);

  static String getValue(String code) => WoxActionsTypeEnum.values.firstWhere((activity) => activity.code == code).value;
}
