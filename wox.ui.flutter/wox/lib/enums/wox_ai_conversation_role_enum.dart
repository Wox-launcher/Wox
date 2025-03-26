typedef WoxAIChatConversationRole = String;

enum WoxAIChatConversationRoleEnum {
  WOX_AIChat_CONVERSATION_ROLE_USER("user", "user"),
  WOX_AIChat_CONVERSATION_ROLE_AI("ai", "ai");

  final String code;
  final String value;

  const WoxAIChatConversationRoleEnum(this.code, this.value);

  static String getValue(String code) => WoxAIChatConversationRoleEnum.values.firstWhere((activity) => activity.code == code).value;
}
