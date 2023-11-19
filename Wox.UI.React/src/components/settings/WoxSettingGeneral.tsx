import React, { useImperativeHandle, useRef, useState } from "react"
import styled from "styled-components"
import { Box, Checkbox, Skeleton } from "@mui/material"
import { WoxSettingHelper } from "../../utils/WoxSettingHelper"
import { useHotkeys } from "react-hotkeys-hook"

export type WoxSettingGeneralRefHandler = {
  initialize: () => void
}

export type WoxSettingGeneralProps = {}

export default React.forwardRef((_props: WoxSettingGeneralProps, ref: React.Ref<WoxSettingGeneralRefHandler>) => {
  const [loading, setLoading] = useState(true)
  const [mainHotkey, setMainHotkey] = useState<string[]>([])
  const [selectionHotkey, setSelectionHotkey] = useState<string[]>([])
  const [usePinYin, setUsePinYin] = useState(false)
  const [hideOnLostFocus, setHideOnLostFocus] = useState(false)
  const [mainHotKeyFocus, setMainHotKeyFocus] = useState(false)
  const [selectionHotKeyFocus, setSelectionHotKeyFocus] = useState(false)
  const updatingSetting = useRef(false)
  const hotKeyArray: string[] = ["control", "option", "shift", "command", "space"]
  const keyMap: { [key: string]: string } = {
    "control": "⌃",
    "option": "⌥",
    "shift": "⇧",
    "command": "⌘",
    "space": "Space"
  }

  const updateSetting = async () => {
    const setting = WoxSettingHelper.getInstance().getSetting()
    setting.SelectionHotkey = { MacValue: selectionHotkey.join("+") }
    setting.MainHotkey = { MacValue: mainHotkey.join("+") }
    const resp = await WoxSettingHelper.getInstance().updateSetting(setting)
    console.log(resp)
  }

  useHotkeys(["ctrl", "alt", "shift", "meta", "ctrl+alt", "ctrl+shift", "ctrl+meta", "ctrl+alt+shift",
    "ctrl+alt+meta", "ctrl+alt+shift+meta", "alt+shift", "alt+meta", "alt+shift+meta", "shift+meta"], (_, handler) => {
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
      setMainHotkey(hotKeyCombinations)
    }
    if (selectionHotKeyFocus) {
      setSelectionHotkey(hotKeyCombinations)
    }
    if (!updatingSetting.current) {
      setTimeout(() => {
        updateSetting().then(_ => {
          updatingSetting.current = false
        })
      }, 500)
    }
    updatingSetting.current = true
  })

  useImperativeHandle(ref, () => ({
    initialize: () => {
      const setting = WoxSettingHelper.getInstance().getSetting()
      // @ts-ignore
      setMainHotkey(setting.MainHotkey.split("+"))
      // @ts-ignore
      setSelectionHotkey(setting.SelectionHotkey.split("+"))
      setUsePinYin(setting.UsePinYin)
      setHideOnLostFocus(setting.HideOnLostFocus)
      setLoading(false)
    }
  }))

  return <Style>
    {loading &&
      <Box>
        <Skeleton />
        <Skeleton animation="wave" />
        <Skeleton animation={false} />
      </Box>}
    {!loading && <>
      <div className={"wox-setting-item"}>
        <div className={"setting-item-label"}>Wox Main Hotkey:</div>
        <div className={"setting-item-content"}>
          <div className={"setting-item-detail"}>
            <div className={`hot-key-area ${mainHotKeyFocus ? "hot-key-focus" : ""}`} onClick={() => {
              setMainHotKeyFocus(true)
              setSelectionHotKeyFocus(false)
            }}>
              {hotKeyArray.map((key, index) => {
                return <span key={index} className={`hot-key-item ${mainHotkey.includes(key) ? "hot-key-item-include" : ""}`}>{keyMap[key]}</span>
              })}
            </div>
          </div>
        </div>
      </div>
      <div className={"wox-setting-item"}>
        <div className={"setting-item-label"}>Wox Selection Hotkey:</div>
        <div className={"setting-item-content"}>
          <div className={"setting-item-detail"}>
            <div className={`hot-key-area ${selectionHotKeyFocus ? "hot-key-focus" : ""}`} onClick={() => {
              setMainHotKeyFocus(false)
              setSelectionHotKeyFocus(true)
            }}>
              {hotKeyArray.map((key, index) => {
                return <span key={index} className={`hot-key-item ${selectionHotkey.includes(key) ? "hot-key-item-include" : ""}`}>{keyMap[key]}</span>
              })}
            </div>
          </div>
        </div>
      </div>
      <div className={"wox-setting-item"}>
        <div className={"setting-item-label"}>Use PinYin:</div>
        <div className={"setting-item-content"}>
          <div className={"setting-item-detail"}><Checkbox defaultChecked={usePinYin} /> Use PinYin For Searching</div>
          <div className={"setting-item-intro"}>If selected, When searching, it converts Chinese into Pinyin and matches it.</div>
        </div>
      </div>
      <div className={"wox-setting-item"}>
        <div className={"setting-item-label"}>Hide On Lost Focus:</div>
        <div className={"setting-item-content"}>
          <div className={"setting-item-detail"}><Checkbox defaultChecked={hideOnLostFocus} /> Hide Wox On Lost Focus</div>
          <div className={"setting-item-intro"}>If selected, When wox lost focus, it will be hidden.</div>
        </div>
      </div>
      <div className={"wox-setting-item"}>
        <div className={"setting-item-label"}>Hide On Start:</div>
        <div className={"setting-item-content"}>
          <div className={"setting-item-detail"}><Checkbox defaultChecked={hideOnLostFocus} /> Hide Wox On Start</div>
          <div className={"setting-item-intro"}>If selected, When wox start, it will be hidden.</div>
        </div>
      </div>
    </>}
  </Style>
})

const Style = styled.div`
  padding: 25px;

  .wox-setting-item {
    display: flex;
    justify-content: center;
    margin-bottom: 10px;

    .setting-item-label, .setting-item-additional {
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

      .MuiCheckbox-root, .Mui-disabled {
        color: #1976d2;
      }

      .MuiOutlinedInput-notchedOutline {
        border-color: #1976d2;
      }
    }
  }
`