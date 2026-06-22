typedef WoxListItemTailTextCategory = String;

const String woxListItemTailTextCategoryDefault = "default";
const String woxListItemTailTextCategoryDanger = "danger";
const String woxListItemTailTextCategoryWarning = "warning";
const String woxListItemTailTextCategorySuccess = "success";

enum WoxListItemTailTextCategoryEnum {
  defaultCategory(woxListItemTailTextCategoryDefault, woxListItemTailTextCategoryDefault),
  danger(woxListItemTailTextCategoryDanger, woxListItemTailTextCategoryDanger),
  warning(woxListItemTailTextCategoryWarning, woxListItemTailTextCategoryWarning),
  success(woxListItemTailTextCategorySuccess, woxListItemTailTextCategorySuccess);

  final String code;
  final String value;

  const WoxListItemTailTextCategoryEnum(this.code, this.value);

  static String ensureCode(String? code) {
    if (code == null || code.isEmpty) {
      return woxListItemTailTextCategoryDefault;
    }

    for (final category in WoxListItemTailTextCategoryEnum.values) {
      if (category.code == code) {
        return category.code;
      }
    }

    return woxListItemTailTextCategoryDefault;
  }
}
