import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_preview_top_status_bar.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/color_util.dart';

class WoxTerminalPreviewView extends StatefulWidget {
  final WoxPreview woxPreview;
  final WoxTheme woxTheme;

  const WoxTerminalPreviewView({super.key, required this.woxPreview, required this.woxTheme});

  @override
  State<WoxTerminalPreviewView> createState() => _WoxTerminalPreviewViewState();
}

class _LocalMatch {
  final int start;
  final int end;

  const _LocalMatch({required this.start, required this.end});
}

class _WoxTerminalPreviewViewState extends State<WoxTerminalPreviewView> {
  static const int maxLocalBytes = 2 * 1024 * 1024;
  static const int highlightRenderThreshold = 2 * 1024 * 1024;
  static const int initialVisibleLines = 100;
  static const int loadMoreBytes = 64 * 1024;
  static const double topLoadTriggerOffset = 16;

  final controller = Get.find<WoxLauncherController>();
  final scrollController = ScrollController();
  final searchController = TextEditingController();
  final searchFocusNode = FocusNode();

  StreamSubscription<Map<String, dynamic>>? chunkSubscription;
  StreamSubscription<Map<String, dynamic>>? stateSubscription;
  Worker? findWorker;

  String sessionId = "";
  String terminalCommand = "";
  String terminalText = "";
  bool sessionRunning = false;
  int baseCursor = 0;
  int currentCursor = 0;
  bool autoFollow = true;
  bool useInitialLineWindow = true;
  bool loadingHistory = false;
  int lastLoadRequestCursor = -1;

  bool showSearch = false;
  bool caseSensitive = false;
  int matchStart = -1;
  int matchEnd = -1;
  List<_LocalMatch> localMatches = const [];
  int currentLocalMatchIndex = -1;

  @override
  void initState() {
    super.initState();
    scrollController.addListener(onScrollChanged);
    bindSession();
    findWorker = ever<int>(controller.terminalFindTrigger, (_) {
      if (mounted) {
        openSearchBar();
      }
    });
  }

  @override
  void didUpdateWidget(covariant WoxTerminalPreviewView oldWidget) {
    super.didUpdateWidget(oldWidget);
    final nextSessionId = controller.getTerminalSessionId(widget.woxPreview);
    if (nextSessionId != sessionId) {
      bindSession();
    }
  }

  @override
  void dispose() {
    findWorker?.dispose();
    chunkSubscription?.cancel();
    stateSubscription?.cancel();
    if (sessionId.isNotEmpty) {
      unawaited(controller.unsubscribeTerminalSession(const UuidV4().generate(), sessionId));
    }
    searchController.dispose();
    searchFocusNode.dispose();
    scrollController.dispose();
    super.dispose();
  }

  String getSearchHotkeyTooltip() {
    return controller.previewSearchHotkeyLabel;
  }

  void bindSession() {
    final traceId = const UuidV4().generate();
    final nextSessionId = controller.getTerminalSessionId(widget.woxPreview);
    if (nextSessionId == sessionId) {
      return;
    }

    if (sessionId.isNotEmpty) {
      unawaited(controller.unsubscribeTerminalSession(traceId, sessionId));
    }
    chunkSubscription?.cancel();
    stateSubscription?.cancel();

    sessionId = nextSessionId;
    terminalCommand = controller.getTerminalCommand(widget.woxPreview);
    final status = controller.getTerminalStatus(widget.woxPreview);
    sessionRunning = status == "running";
    terminalText = "";
    baseCursor = 0;
    currentCursor = 0;
    autoFollow = true;
    useInitialLineWindow = true;
    loadingHistory = false;
    lastLoadRequestCursor = -1;
    matchStart = -1;
    matchEnd = -1;
    localMatches = const [];
    currentLocalMatchIndex = -1;

    if (sessionId.isEmpty) {
      setState(() {});
      return;
    }

    chunkSubscription = controller.terminalChunkStream(sessionId).listen(onChunkReceived);
    stateSubscription = controller.terminalStateStream(sessionId).listen(onStateReceived);
    unawaited(controller.subscribeTerminalSession(traceId, sessionId, cursor: -1));
    setState(() {});
  }

  void onStateReceived(Map<String, dynamic> data) {
    setState(() {
      final command = data["Command"]?.toString() ?? "";
      if (command.isNotEmpty) {
        terminalCommand = command;
      }
      final rawStatus = data["Status"]?.toString() ?? "";
      sessionRunning = rawStatus == "running";
    });
  }

  void onChunkReceived(Map<String, dynamic> data) {
    final chunk = data["Content"]?.toString() ?? "";
    if (chunk.isEmpty) {
      return;
    }

    final hadScrollClients = scrollController.hasClients;
    final previousPixels = hadScrollClients ? scrollController.position.pixels : 0.0;
    final previousMaxExtent = hadScrollClients ? scrollController.position.maxScrollExtent : 0.0;
    final wasLoadingHistory = loadingHistory;

    final start = (data["CursorStart"] as num?)?.toInt() ?? currentCursor;
    final end = (data["CursorEnd"] as num?)?.toInt() ?? (start + chunk.length);
    final truncated = data["Truncated"] == true;

    setState(() {
      if (terminalText.isEmpty || truncated || start < baseCursor) {
        baseCursor = start;
        terminalText = chunk;
      } else {
        final offset = start - baseCursor;
        if (offset >= terminalText.length) {
          terminalText += chunk;
        } else if (offset >= 0) {
          final overwriteEnd = (offset + chunk.length).clamp(0, terminalText.length);
          final prefix = terminalText.substring(0, offset);
          final suffix = overwriteEnd < terminalText.length ? terminalText.substring(overwriteEnd) : "";
          terminalText = prefix + chunk + suffix;
        } else {
          baseCursor = start;
          terminalText = chunk;
        }
      }

      if (end > currentCursor) {
        currentCursor = end;
      }

      if (terminalText.length > maxLocalBytes) {
        final trim = terminalText.length - maxLocalBytes;
        terminalText = terminalText.substring(trim);
        baseCursor += trim;
      }

      if (useInitialLineWindow) {
        trimToRecentLines();
      }

      if (wasLoadingHistory) {
        loadingHistory = false;
      }
    });

    if (showSearch && searchController.text.trim().isNotEmpty) {
      setState(() {
        rebuildSearchMatches(preserveCurrent: true);
      });
    }

    if (wasLoadingHistory) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted || !scrollController.hasClients) {
          return;
        }
        final newMaxExtent = scrollController.position.maxScrollExtent;
        final delta = newMaxExtent - previousMaxExtent;
        final target = (previousPixels + delta).clamp(0.0, newMaxExtent);
        scrollController.jumpTo(target);
      });
      return;
    }

    if (autoFollow) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted || !scrollController.hasClients) {
          return;
        }
        scrollController.jumpTo(scrollController.position.maxScrollExtent);
      });
    }
  }

  void onScrollChanged() {
    if (!scrollController.hasClients) {
      return;
    }
    final maxExtent = scrollController.position.maxScrollExtent;
    final distanceToBottom = maxExtent - scrollController.position.pixels;
    autoFollow = distanceToBottom <= 24;
    if (scrollController.position.pixels <= topLoadTriggerOffset) {
      requestLoadMoreHistory();
    }
  }

  void trimToRecentLines() {
    if (terminalText.isEmpty) {
      return;
    }

    int foundLines = 0;
    int index = terminalText.length;
    while (index > 0 && foundLines < initialVisibleLines) {
      index = terminalText.lastIndexOf('\n', index - 1);
      if (index < 0) {
        break;
      }
      foundLines++;
    }

    if (index > 0) {
      final trimBytes = index + 1;
      terminalText = terminalText.substring(trimBytes);
      baseCursor += trimBytes;
    }
  }

  void requestLoadMoreHistory() {
    if (sessionId.isEmpty || loadingHistory || baseCursor <= 0) {
      return;
    }

    final targetCursor = (baseCursor - loadMoreBytes).clamp(0, baseCursor - 1).toInt();
    if (targetCursor == lastLoadRequestCursor) {
      return;
    }

    setState(() {
      loadingHistory = true;
      useInitialLineWindow = false;
      lastLoadRequestCursor = targetCursor;
    });
    unawaited(
      controller.subscribeTerminalSession(const UuidV4().generate(), sessionId, cursor: targetCursor).timeout(const Duration(seconds: 2), onTimeout: () {}).whenComplete(() {
        if (!mounted) {
          return;
        }
        if (loadingHistory) {
          setState(() {
            loadingHistory = false;
          });
        }
      }),
    );
  }

  void openSearchBar() {
    setState(() {
      showSearch = true;
      rebuildSearchMatches(preserveCurrent: false);
    });
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        searchFocusNode.requestFocus();
      }
    });
  }

  void closeSearchBar({required bool focusQueryBox}) {
    setState(() {
      showSearch = false;
      localMatches = const [];
      currentLocalMatchIndex = -1;
      matchStart = -1;
      matchEnd = -1;
    });
    if (focusQueryBox) {
      unawaited(controller.focusQueryBox());
    }
  }

  void rebuildSearchMatches({required bool preserveCurrent}) {
    final keyword = searchController.text.trim();
    if (keyword.isEmpty || terminalText.isEmpty) {
      localMatches = const [];
      currentLocalMatchIndex = -1;
      matchStart = -1;
      matchEnd = -1;
      return;
    }

    final source = caseSensitive ? terminalText : terminalText.toLowerCase();
    final query = caseSensitive ? keyword : keyword.toLowerCase();

    final previousAbsoluteStart = currentLocalMatchIndex >= 0 && currentLocalMatchIndex < localMatches.length ? baseCursor + localMatches[currentLocalMatchIndex].start : -1;

    final matches = <_LocalMatch>[];
    int from = 0;
    while (from <= source.length) {
      final index = source.indexOf(query, from);
      if (index < 0) {
        break;
      }
      matches.add(_LocalMatch(start: index, end: index + keyword.length));
      from = index + (query.isEmpty ? 1 : query.length);
    }

    localMatches = matches;
    if (localMatches.isEmpty) {
      currentLocalMatchIndex = -1;
      matchStart = -1;
      matchEnd = -1;
      return;
    }

    if (preserveCurrent && previousAbsoluteStart >= baseCursor) {
      final preservedIndex = localMatches.indexWhere((match) => baseCursor + match.start == previousAbsoluteStart);
      if (preservedIndex >= 0) {
        currentLocalMatchIndex = preservedIndex;
      } else {
        final nextIndex = localMatches.indexWhere((match) => baseCursor + match.start > previousAbsoluteStart);
        currentLocalMatchIndex = nextIndex >= 0 ? nextIndex : localMatches.length - 1;
      }
    } else if (currentLocalMatchIndex < 0 || currentLocalMatchIndex >= localMatches.length) {
      currentLocalMatchIndex = 0;
    }

    syncCurrentMatchRange();
  }

  void syncCurrentMatchRange() {
    if (currentLocalMatchIndex < 0 || currentLocalMatchIndex >= localMatches.length) {
      matchStart = -1;
      matchEnd = -1;
      return;
    }

    final current = localMatches[currentLocalMatchIndex];
    matchStart = baseCursor + current.start;
    matchEnd = baseCursor + current.end;
  }

  void searchNext({required bool backward}) {
    final keyword = searchController.text.trim();
    if (keyword.isEmpty) {
      return;
    }

    setState(() {
      rebuildSearchMatches(preserveCurrent: true);
      if (localMatches.isEmpty) {
        return;
      }

      if (currentLocalMatchIndex < 0 || currentLocalMatchIndex >= localMatches.length) {
        currentLocalMatchIndex = backward ? localMatches.length - 1 : 0;
      } else if (backward) {
        currentLocalMatchIndex = (currentLocalMatchIndex - 1 + localMatches.length) % localMatches.length;
      } else {
        currentLocalMatchIndex = (currentLocalMatchIndex + 1) % localMatches.length;
      }

      syncCurrentMatchRange();
    });

    jumpToMatch();
  }

  void jumpToMatch() {
    if (!scrollController.hasClients || terminalText.isEmpty || matchStart < baseCursor || matchStart > baseCursor + terminalText.length) {
      return;
    }

    final localOffset = (matchStart - baseCursor).clamp(0, terminalText.length);
    final ratio = terminalText.isEmpty ? 0.0 : localOffset / terminalText.length;
    final target = scrollController.position.maxScrollExtent * ratio;
    scrollController.animateTo(target, duration: const Duration(milliseconds: 140), curve: Curves.easeOut);
  }

  Widget buildTerminalText() {
    if (localMatches.isNotEmpty && terminalText.length <= highlightRenderThreshold) {
      final fontColor = safeFromCssColor(widget.woxTheme.previewFontColor);
      const currentMatchBackground = Color(0xFFFBC02D);
      const otherMatchBackground = Color(0xFFFFF59D);

      final spans = <TextSpan>[];
      int cursor = 0;
      for (int i = 0; i < localMatches.length; i++) {
        final match = localMatches[i];
        if (match.start < cursor || match.end > terminalText.length) {
          continue;
        }

        if (cursor < match.start) {
          spans.add(TextSpan(text: terminalText.substring(cursor, match.start)));
        }
        spans.add(
          TextSpan(
            text: terminalText.substring(match.start, match.end),
            style: TextStyle(color: Colors.black.withValues(alpha: 0.95), backgroundColor: i == currentLocalMatchIndex ? currentMatchBackground : otherMatchBackground),
          ),
        );
        cursor = match.end;
      }

      if (cursor < terminalText.length) {
        spans.add(TextSpan(text: terminalText.substring(cursor)));
      }

      return SelectableText.rich(TextSpan(style: TextStyle(color: fontColor), children: spans));
    }

    return SelectableText(terminalText, style: TextStyle(color: safeFromCssColor(widget.woxTheme.previewFontColor)));
  }

  Widget buildSearchBar() {
    if (!showSearch) {
      return const SizedBox();
    }

    final fontColor = safeFromCssColor(widget.woxTheme.previewFontColor);
    final backgroundColor = safeFromCssColor(widget.woxTheme.queryBoxBackgroundColor).withValues(alpha: 0.5);
    final borderColor = safeFromCssColor(widget.woxTheme.previewSplitLineColor);
    final countText = localMatches.isEmpty ? "0/0" : "${currentLocalMatchIndex + 1}/${localMatches.length}";

    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
      decoration: BoxDecoration(color: backgroundColor, borderRadius: BorderRadius.circular(8), border: Border.all(color: borderColor, width: 1)),
      child: Row(
        children: [
          Expanded(
            child: Focus(
              onKeyEvent: (node, event) {
                if (event is! KeyDownEvent) {
                  return KeyEventResult.ignored;
                }

                final pressedHotkey = WoxHotkey.parseNormalHotkeyFromEvent(event);
                if (pressedHotkey != null &&
                    controller.executeLocalActionByHotkey(
                      const UuidV4().generate(),
                      pressedHotkey,
                      allowedActionIds: {WoxLauncherController.localActionTogglePreviewFullscreenId},
                    )) {
                  searchFocusNode.requestFocus();
                  return KeyEventResult.handled;
                }

                if (event.logicalKey == LogicalKeyboardKey.escape) {
                  closeSearchBar(focusQueryBox: true);
                  return KeyEventResult.handled;
                }

                if (event.logicalKey == LogicalKeyboardKey.enter || event.logicalKey == LogicalKeyboardKey.numpadEnter) {
                  searchNext(backward: HardwareKeyboard.instance.isShiftPressed);
                  searchFocusNode.requestFocus();
                  return KeyEventResult.handled;
                }

                return KeyEventResult.ignored;
              },
              child: TextField(
                controller: searchController,
                focusNode: searchFocusNode,
                style: TextStyle(color: fontColor),
                textInputAction: TextInputAction.search,
                cursorColor: safeFromCssColor(widget.woxTheme.queryBoxCursorColor),
                onChanged: (_) {
                  setState(() {
                    rebuildSearchMatches(preserveCurrent: false);
                  });
                },
                onSubmitted: (_) {
                  searchNext(backward: false);
                  searchFocusNode.requestFocus();
                },
                decoration: InputDecoration(
                  isDense: true,
                  contentPadding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
                  filled: true,
                  fillColor: safeFromCssColor(widget.woxTheme.appBackgroundColor).withValues(alpha: 0.2),
                  border: OutlineInputBorder(borderRadius: BorderRadius.circular(6), borderSide: BorderSide(color: borderColor)),
                  enabledBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(6), borderSide: BorderSide(color: borderColor)),
                  focusedBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(6), borderSide: BorderSide(color: safeFromCssColor(widget.woxTheme.queryBoxCursorColor))),
                ),
              ),
            ),
          ),
          const SizedBox(width: 8),
          Text(countText, style: TextStyle(color: fontColor.withValues(alpha: 0.85), fontSize: 12, fontWeight: FontWeight.w500)),
          IconButton(onPressed: () => searchNext(backward: true), icon: const Icon(Icons.keyboard_arrow_up), color: fontColor.withValues(alpha: 0.9)),
          IconButton(onPressed: () => searchNext(backward: false), icon: const Icon(Icons.keyboard_arrow_down), color: fontColor.withValues(alpha: 0.9)),
          IconButton(
            onPressed: () {
              setState(() {
                caseSensitive = !caseSensitive;
                rebuildSearchMatches(preserveCurrent: false);
              });
            },
            icon: Text("Aa", style: TextStyle(color: fontColor.withValues(alpha: 0.9), fontWeight: caseSensitive ? FontWeight.bold : FontWeight.normal)),
          ),
          IconButton(onPressed: () => closeSearchBar(focusQueryBox: false), icon: const Icon(Icons.close), color: fontColor.withValues(alpha: 0.9)),
        ],
      ),
    );
  }

  Widget buildTopStatusBar() {
    final commandText = terminalCommand.isNotEmpty ? terminalCommand : "-";
    final statusDotColor = sessionRunning ? Colors.green : Colors.grey;
    final fontColor = safeFromCssColor(widget.woxTheme.previewFontColor);

    return Obx(() {
      final isFullscreen = controller.isPreviewFullscreen.value;
      return WoxPreviewTopStatusBar(
        woxTheme: widget.woxTheme,
        leading: Container(width: 8, height: 8, decoration: BoxDecoration(color: statusDotColor, shape: BoxShape.circle)),
        title: Text(commandText, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: fontColor, fontSize: 16, fontWeight: FontWeight.w600, height: 1.1)),
        trailing: loadingHistory ? SizedBox(width: 12, height: 12, child: CircularProgressIndicator(strokeWidth: 2, color: fontColor.withValues(alpha: 0.8))) : null,
        actions: [
          WoxPreviewTopStatusBarAction(tooltip: getSearchHotkeyTooltip(), onPressed: openSearchBar, icon: const Icon(Icons.search)),
          WoxPreviewTopStatusBarAction(
            tooltip: controller.previewFullscreenHotkeyLabel,
            onPressed: () {
              controller.togglePreviewFullscreen(const UuidV4().generate());
            },
            icon: Icon(isFullscreen ? Icons.fullscreen_exit : Icons.fullscreen),
          ),
        ],
      );
    });
  }

  @override
  Widget build(BuildContext context) {
    return Focus(
      onKeyEvent: (node, event) {
        if (event is! KeyDownEvent || event.logicalKey != LogicalKeyboardKey.escape) {
          return KeyEventResult.ignored;
        }

        if (showSearch) {
          closeSearchBar(focusQueryBox: true);
        } else {
          unawaited(controller.focusQueryBox());
        }
        return KeyEventResult.handled;
      },
      child: Column(
        children: [
          buildTopStatusBar(),
          buildSearchBar(),
          Expanded(
            child: Scrollbar(
              thumbVisibility: true,
              controller: scrollController,
              child: SingleChildScrollView(controller: scrollController, child: Align(alignment: Alignment.centerLeft, child: buildTerminalText())),
            ),
          ),
        ],
      ),
    );
  }
}
