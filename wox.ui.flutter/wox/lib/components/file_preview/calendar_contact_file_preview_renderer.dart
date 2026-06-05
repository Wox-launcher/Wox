import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:path/path.dart' as path;
import 'package:wox/components/file_preview/file_info_preview.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

class CalendarContactFilePreviewRenderer implements WoxFilePreviewRenderer {
  @override
  bool supports(String fileExtension) {
    return fileExtension == "ics" || fileExtension == "vcf";
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final file = File(context.filePath);
    if (!file.existsSync()) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_calendar_contact_not_found", {"path": context.filePath})));
    }

    if (context.fileExtension == "ics") {
      return WoxFilePreviewResult(content: _buildCalendarPreview(file, context.tr));
    }
    return WoxFilePreviewResult(content: _buildContactPreview(file, context.tr));
  }
}

Widget _buildCalendarPreview(File file, WoxFilePreviewTranslationFormatter tr) {
  final lines = _parseStructuredTextLines(file);
  final event = _firstComponent(lines, "VEVENT");
  final summary = _propertyValue(event, "SUMMARY");
  final location = _propertyValue(event, "LOCATION");
  final startsAt = _formatCalendarDate(_propertyValue(event, "DTSTART"));
  final endsAt = _formatCalendarDate(_propertyValue(event, "DTEND"));
  final organizer = _cleanMailto(_propertyValue(event, "ORGANIZER"));
  final description = _propertyValue(event, "DESCRIPTION");
  final typeLabel = tr("ui_file_preview_type_calendar");

  return WoxFileInfoPreview(
    icon: Icons.event_note_rounded,
    fileIconPath: file.path,
    accent: const Color(0xFF2563EB),
    title: summary.isNotEmpty ? summary : path.basename(file.path),
    subtitle: startsAt.isNotEmpty ? startsAt : typeLabel,
    properties: [
      ...buildWoxFilePreviewCommonProperties(file, typeLabel: typeLabel, tr: tr),
      if (startsAt.isNotEmpty) WoxFilePreviewProperty(label: tr("ui_file_preview_calendar_start"), value: startsAt),
      if (endsAt.isNotEmpty) WoxFilePreviewProperty(label: tr("ui_file_preview_calendar_end"), value: endsAt),
      if (location.isNotEmpty) WoxFilePreviewProperty(label: tr("ui_file_preview_calendar_location"), value: location),
      if (organizer.isNotEmpty) WoxFilePreviewProperty(label: tr("ui_file_preview_calendar_organizer"), value: organizer),
    ],
    sections: [if (description.isNotEmpty) _buildTextSection(title: tr("ui_file_preview_calendar_description"), text: description)],
  );
}

Widget _buildContactPreview(File file, WoxFilePreviewTranslationFormatter tr) {
  final lines = _parseStructuredTextLines(file);
  final card = _firstComponent(lines, "VCARD");
  final fullName = _propertyValue(card, "FN");
  final organization = _propertyValue(card, "ORG");
  final title = _propertyValue(card, "TITLE");
  final phones = _propertyValues(card, "TEL");
  final emails = _propertyValues(card, "EMAIL");
  final address = _propertyValue(card, "ADR").replaceAll(";", " ").replaceAll(RegExp(r"\s+"), " ").trim();
  final url = _propertyValue(card, "URL");
  final note = _propertyValue(card, "NOTE");
  final typeLabel = tr("ui_file_preview_type_contact");

  return WoxFileInfoPreview(
    icon: Icons.contact_page_rounded,
    fileIconPath: file.path,
    accent: const Color(0xFF10B981),
    title: fullName.isNotEmpty ? fullName : path.basename(file.path),
    subtitle: organization.isNotEmpty ? organization : typeLabel,
    properties: [
      ...buildWoxFilePreviewCommonProperties(file, typeLabel: typeLabel, tr: tr),
      if (title.isNotEmpty) WoxFilePreviewProperty(label: tr("ui_file_preview_contact_title"), value: title),
      if (organization.isNotEmpty) WoxFilePreviewProperty(label: tr("ui_file_preview_contact_organization"), value: organization),
      if (phones.isNotEmpty) WoxFilePreviewProperty(label: tr("ui_file_preview_contact_phone"), value: phones.join(", ")),
      if (emails.isNotEmpty) WoxFilePreviewProperty(label: tr("ui_file_preview_contact_email"), value: emails.join(", ")),
      if (address.isNotEmpty) WoxFilePreviewProperty(label: tr("ui_file_preview_contact_address"), value: address),
      if (url.isNotEmpty) WoxFilePreviewProperty(label: tr("ui_file_preview_contact_url"), value: url),
    ],
    sections: [if (note.isNotEmpty) _buildTextSection(title: tr("ui_file_preview_contact_note"), text: note)],
  );
}

WoxFilePreviewSection _buildTextSection({required String title, required String text}) {
  return WoxFilePreviewSection(
    title: title,
    child: Padding(
      padding: const EdgeInsets.all(12),
      child: Text(text, style: TextStyle(color: getThemeTextColor(), fontSize: WoxInterfaceSizeUtil.instance.current.resultSubtitleFontSize, height: 1.35)),
    ),
  );
}

List<String> _parseStructuredTextLines(File file) {
  final text = _readTextFile(file);
  final lines = <String>[];
  for (final rawLine in const LineSplitter().convert(text)) {
    if ((rawLine.startsWith(" ") || rawLine.startsWith("\t")) && lines.isNotEmpty) {
      lines[lines.length - 1] = "${lines.last}${rawLine.substring(1)}";
      continue;
    }
    lines.add(rawLine.trimRight());
  }
  return lines;
}

List<String> _firstComponent(List<String> lines, String component) {
  final values = <String>[];
  var inComponent = false;
  for (final line in lines) {
    final upper = line.toUpperCase();
    if (upper == "BEGIN:$component") {
      inComponent = true;
      continue;
    }
    if (upper == "END:$component" && inComponent) {
      break;
    }
    if (inComponent) {
      values.add(line);
    }
  }
  return values.isEmpty ? lines : values;
}

String _propertyValue(List<String> lines, String key) {
  final values = _propertyValues(lines, key);
  return values.isEmpty ? "" : values.first;
}

List<String> _propertyValues(List<String> lines, String key) {
  final values = <String>[];
  for (final line in lines) {
    final colon = line.indexOf(":");
    if (colon <= 0) {
      continue;
    }
    final name = line.substring(0, colon).split(";").first.toUpperCase();
    if (name == key) {
      values.add(_unescapeStructuredText(line.substring(colon + 1)));
    }
  }
  return values.where((value) => value.trim().isNotEmpty).toList();
}

String _formatCalendarDate(String value) {
  final trimmed = value.trim();
  if (trimmed.length == 8 && RegExp(r"^\d{8}$").hasMatch(trimmed)) {
    return "${trimmed.substring(0, 4)}-${trimmed.substring(4, 6)}-${trimmed.substring(6, 8)}";
  }

  final match = RegExp(r"^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})Z?$").firstMatch(trimmed);
  if (match == null) {
    return trimmed;
  }
  return "${match.group(1)}-${match.group(2)}-${match.group(3)} ${match.group(4)}:${match.group(5)}";
}

String _cleanMailto(String value) {
  final trimmed = value.trim();
  return trimmed.toLowerCase().startsWith("mailto:") ? trimmed.substring(7) : trimmed;
}

String _unescapeStructuredText(String value) {
  return value.replaceAll(r"\n", "\n").replaceAll(r"\N", "\n").replaceAll(r"\,", ",").replaceAll(r"\;", ";").replaceAll("\\\\", "\\").trim();
}

String _readTextFile(File file) {
  final bytes = file.readAsBytesSync();
  if (bytes.length >= 3 && bytes[0] == 0xEF && bytes[1] == 0xBB && bytes[2] == 0xBF) {
    return utf8.decode(bytes.sublist(3), allowMalformed: true);
  }
  return utf8.decode(bytes, allowMalformed: true);
}
