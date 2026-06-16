# Tray Query Anchor Design

## Problem
The backend currently computes a tray query window top-left using an estimated initial height. For preview-only tray queries on Windows, Flutter often knows a much larger real initial height before `ShowApp`, so the final window opens too high or otherwise detached from the tray anchor.

## Approved Approach
Keep backend ownership of tray icon monitor detection and anchor selection, but move final top-left calculation into Flutter. The backend will send tray anchor metadata for tray-query flows. Flutter will derive the final window top-left from the anchor and the actual `targetHeight` it computes for that show action.

## Boundaries
- Backend: identify tray-query anchor and package it in the show payload.
- Flutter UI: compute final top-left from anchor, width, and real target height.
- Non-tray flows: unchanged.

## Expected Result
Tray-query windows stay aligned with the tray icon even when preview state, toolbar visibility, or hidden query box changes the true initial height.
