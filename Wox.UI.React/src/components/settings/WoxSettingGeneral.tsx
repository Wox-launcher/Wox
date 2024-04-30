import React, { useImperativeHandle, useRef, useState } from "react"
import styled from "styled-components"
import { Box, Checkbox, FormControl, FormControlLabel, MenuItem, Select, Skeleton } from "@mui/material"
import { WoxSettingHelper } from "../../utils/WoxSettingHelper"
import { useHotkeys } from "react-hotkeys-hook"
import { UpdateSetting } from "../../entity/Setting.typings"

export type WoxSettingGeneralRefHandler = {
  initialize: () => void
}

export type WoxSettingGeneralProps = {}

export default React.forwardRef((_props: WoxSettingGeneralProps, ref: React.Ref<WoxSettingGeneralRefHandler>) => {
  const currentMainHotKey = useRef<string[]>([])
  const currentSelectionHotKey = useRef<string[]>([])
  const [loading, setLoading] = useState(true)
  const [mainHotkey, setMainHotkey] = useState<string[]>([])
  const [selectionHotkey, setSelectionHotkey] = useState<string[]>([])
  const [usePinYin, setUsePinYin] = useState(false)
  const [hideOnLostFocus, setHideOnLostFocus] = useState(false)
  const [mainHotKeyFocus, setMainHotKeyFocus] = useState(false)
  const [selectionHotKeyFocus, setSelectionHotKeyFocus] = useState(false)
  const [lastQueryMode, setLastQueryMode] = useState("empty")
  const updatingSetting = useRef(false)
  const hotKeyArray: string[] = ["control", "option", "shift", "command", "space"]
  const keyMap: { [key: string]: string } = {
    control: "⌃",
    option: "⌥",
    shift: "⇧",
    command: "⌘",
    space: "Space"
  }

  const executeSettingUpdate = async (updateSetting: UpdateSetting) => {
    await WoxSettingHelper.getInstance().updateSetting(updateSetting)
  }

  useHotkeys(
    ["ctrl", "alt", "shift", "meta", "ctrl+alt", "ctrl+shift", "ctrl+meta", "ctrl+alt+shift", "ctrl+alt+meta", "ctrl+alt+shift+meta", "alt+shift", "alt+meta", "alt+shift+meta", "shift+meta"],
    (_, handler) => {
      const hotKeyCombinations = ["space"]
      if (handler.ctrl) {
        hotKeyCombinations.push("control")
      }
      if (handler.alt) {
        hotKeyCombinations.push("option")
      }
      if (handler.shift) {
        hotKeyCombinations.push("shift")
      }
      if (handler.meta) {
        hotKeyCombinations.push("command")
      }
      if (mainHotKeyFocus) {
        currentMainHotKey.current = hotKeyCombinations
        setMainHotkey(currentMainHotKey.current)
      }
      if (selectionHotKeyFocus) {
        currentSelectionHotKey.current = hotKeyCombinations
        setSelectionHotkey(currentSelectionHotKey.current)
      }
      if (!updatingSetting.current) {
        setTimeout(() => {
          const setting = mainHotKeyFocus ? { Key: "MainHotkey", Value: currentMainHotKey.current.join("+") } : {
            Key: "SelectionHotkey",
            Value: currentSelectionHotKey.current.join("+")
          }
          executeSettingUpdate(setting).then(_ => {
            updatingSetting.current = false
          })
        }, 500)
      }
      updatingSetting.current = true
    }
  )

  useImperativeHandle(ref, () => ({
    initialize: () => {
      const setting = WoxSettingHelper.getInstance().getSetting()
      currentMainHotKey.current = setting.MainHotkey.split("+")
      currentSelectionHotKey.current = setting.SelectionHotkey.split("+")
      setMainHotkey(currentMainHotKey.current)
      setSelectionHotkey(currentSelectionHotKey.current)
      setUsePinYin(setting.UsePinYin)
      setHideOnLostFocus(setting.HideOnLostFocus)
      setLoading(false)
    }
  }))

  return (
    <Style>
      {loading && (
        <Box>
          <Skeleton />
          <Skeleton animation="wave" />
          <Skeleton animation={false} />
        </Box>
      )}
      {!loading && (
        <>
          <div className={"wox-setting-item"}>
            <div className={"setting-item-label"}>Wox Main Hotkey:</div>
            <div className={"setting-item-content"}>
              <div className={"setting-item-detail"}>
                <div
                  className={`hot-key-area ${mainHotKeyFocus ? "hot-key-focus" : ""}`}
                  onClick={() => {
                    setMainHotKeyFocus(true)
                    setSelectionHotKeyFocus(false)
                  }}
                >
                  {hotKeyArray.map((key, index) => {
                    return (
                      <span key={index} className={`hot-key-item ${mainHotkey.includes(key) ? "hot-key-item-include" : ""}`}>
                        {keyMap[key]}
                      </span>
                    )
                  })}
                </div>
              </div>
            </div>
          </div>
          <div className={"wox-setting-item"}>
            <div className={"setting-item-label"}>Wox Selection Hotkey:</div>
            <div className={"setting-item-content"}>
              <div className={"setting-item-detail"}>
                <div
                  className={`hot-key-area ${selectionHotKeyFocus ? "hot-key-focus" : ""}`}
                  onClick={() => {
                    setMainHotKeyFocus(false)
                    setSelectionHotKeyFocus(true)
                  }}
                >
                  {hotKeyArray.map((key, index) => {
                    return (
                      <span key={index} className={`hot-key-item ${selectionHotkey.includes(key) ? "hot-key-item-include" : ""}`}>
                        {keyMap[key]}
                      </span>
                    )
                  })}
                </div>
              </div>
            </div>
          </div>
          <div className={"wox-setting-item"}>
            <div className={"setting-item-label"} style={{ paddingTop: "6px" }}>
              Last Query Model:
            </div>
            <div className={"setting-item-content"}>
              <div className={"setting-item-detail"}>
                <FormControl sx={{ m: 1, minWidth: 220 }} size="small">
                  <Select
                    value={lastQueryMode}
                    onChange={async event => {
                      setLastQueryMode(event.target.value as string)
                      await executeSettingUpdate({ Key: "LastQueryMode", Value: event.target.value })
                    }}
                  >
                    <MenuItem value={"preserve"}>Preserve</MenuItem>
                    <MenuItem value={"empty"}>Empty</MenuItem>
                  </Select>
                </FormControl>
              </div>
              <div className={"setting-item-intro"}>In 'Preserve' model, it will show last query result, when you reopen wox launcher.</div>
            </div>
          </div>
          <div className={"wox-setting-item"}>
            <div className={"setting-item-label"}>Use PinYin:</div>
            <div className={"setting-item-content"}>
              <div className={"setting-item-detail"}>
                <FormControlLabel
                  sx={{ paddingLeft: "8px" }}
                  control={
                    <Checkbox
                      defaultChecked={usePinYin}
                      onChange={async event => {
                        await executeSettingUpdate({ Key: "UsePinYin", Value: event.target.checked + "" })
                      }}
                    />
                  }
                  label="Use PinYin For Searching"
                />
              </div>
              <div className={"setting-item-intro"}>When selected, Wox will convert Chinese into Pinyin.</div>
            </div>
          </div>
          <div className={"wox-setting-item"}>
            <div className={"setting-item-label"}>Hide On Lost Focus:</div>
            <div className={"setting-item-content"}>
              <div className={"setting-item-detail"}>
                <FormControlLabel
                  sx={{ paddingLeft: "8px" }}
                  control={
                    <Checkbox
                      defaultChecked={hideOnLostFocus}
                      onChange={async event => {
                        await executeSettingUpdate({ Key: "HideOnLostFocus", Value: event.target.checked + "" })
                      }}
                    />
                  }
                  label="Hide Wox On Lost Focus"
                />
              </div>
              <div className={"setting-item-intro"}>When selected, Wox will hide on lost focus.</div>
            </div>
          </div>
          <div className={"wox-setting-item"}>
            <div className={"setting-item-label"}>Hide On Start:</div>
            <div className={"setting-item-content"}>
              <div className={"setting-item-detail"}>
                <FormControlLabel
                  sx={{ paddingLeft: "8px" }}
                  control={
                    <Checkbox
                      defaultChecked={hideOnLostFocus}
                      onChange={async event => {
                        await executeSettingUpdate({ Key: "HideOnStart", Value: event.target.checked + "" })
                      }}
                    />
                  }
                  label="Hide Wox On Start"
                />
              </div>
              <div className={"setting-item-intro"}>When selected, Wox will hide on start.</div>
            </div>
          </div>
          <div className={"wox-setting-item"}>
            <div className={"setting-item-label"}>Show Tray:</div>
            <div className={"setting-item-content"}>
              <div className={"setting-item-detail"}>
                <FormControlLabel
                  sx={{ paddingLeft: "8px" }}
                  control={
                    <Checkbox
                      defaultChecked={hideOnLostFocus}
                      onChange={async event => {
                        await executeSettingUpdate({ Key: "ShowTray", Value: event.target.checked + "" })
                      }}
                    />
                  }
                  label="Show Wox Tray Icon"
                />
              </div>
              <div className={"setting-item-intro"}>When selected, Wox will show icon on system tray on start.</div>
            </div>
          </div>
          <div className={"wox-setting-item"}>
            <div className={"setting-item-label"}>Switch Input Method:</div>
            <div className={"setting-item-content"}>
              <div className={"setting-item-detail"}>
                <FormControlLabel
                  sx={{ paddingLeft: "8px" }}
                  control={
                    <Checkbox
                      defaultChecked={hideOnLostFocus}
                      onChange={async event => {
                        await executeSettingUpdate({ Key: "SwitchInputMethodABC", Value: event.target.checked + "" })
                      }}
                    />
                  }
                  label="Switch Input Method To English"
                />
              </div>
              <div className={"setting-item-intro"}>When selected, the input method will be switched to english.</div>
            </div>
          </div>
        </>
      )}
    </Style>
  )
})

const Style = styled.div`
  padding: 25px;

  .wox-setting-item {
    display: flex;
    justify-content: center;
    margin-bottom: 10px;

    .setting-item-label,
    .setting-item-additional {
      flex: 1;
    }

    .setting-item-content {
      flex: 2;
    }

    .setting-item-label {
      line-height: 42px;
      text-align: right;
    }

    .setting-item-content {
      display: flex;
      flex-direction: column;
      justify-content: center;

      .setting-item-detail {
        line-height: 42px;
        vertical-align: middle;

        .hot-key-area {
          border: 1px solid white;
          width: 220px;
          border-radius: 5px;
          margin-left: 10px;
          padding: 0 10px;

          .hot-key-item {
            padding-right: 5px;
            font-size: 18px;
            color: #a1a1a1;
          }

          .hot-key-item-include {
            color: white;
            font-weight: bold;
          }
        }

        .hot-key-focus {
          border: 1px solid #1976d2;
        }
      }

      .setting-item-intro {
        font-size: 12px;
        color: #a1a1a1;
        padding-left: 10px;
      }

      .MuiCheckbox-root,
      .Mui-disabled {
        color: #1976d2;
      }

      .MuiOutlinedInput-notchedOutline {
        border: 1px solid white;
      }

      .MuiInputLabel-root,
      .MuiSelect-select,
      .MuiSelect-icon {
        color: white;
      }
    }
  }
`
