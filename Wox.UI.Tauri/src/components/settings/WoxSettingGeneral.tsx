import React, { useImperativeHandle, useState } from "react"
import styled from "styled-components"
import { Box, Checkbox, Skeleton, TextField } from "@mui/material"
import { WoxSettingHelper } from "../../utils/WoxSettingHelper"

export type WoxSettingGeneralRefHandler = {
  initialize: () => void
}

export type WoxSettingGeneralProps = {}

export default React.forwardRef((_props: WoxSettingGeneralProps, ref: React.Ref<WoxSettingGeneralRefHandler>) => {
  const [loading, setLoading] = useState(true)
  const [mainHotkey, setMainHotkey] = useState("")
  const [selectionHotkey, setSelectionHotkey] = useState("")
  const [usePinYin, setUsePinYin] = useState(false)
  const [hideOnLostFocus, setHideOnLostFocus] = useState(false)

  useImperativeHandle(ref, () => ({
    initialize: () => {
      const setting = WoxSettingHelper.getInstance().getSetting()
      setMainHotkey(setting.MainHotkey)
      setSelectionHotkey(setting.SelectionHotkey)
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
            <TextField
              disabled
              label="Main Hotkey"
              InputLabelProps={{
                shrink: true
              }}
              sx={{ m: 1, width: "25ch" }}
              defaultValue={mainHotkey}
            />
          </div>
        </div>
      </div>
      <div className={"wox-setting-item"}>
        <div className={"setting-item-label"}>Wox Selection Hotkey:</div>
        <div className={"setting-item-content"}>
          <div className={"setting-item-detail"}>
            <TextField
              disabled
              label="Selection Hotkey"
              InputLabelProps={{
                shrink: true
              }}
              sx={{ m: 1, width: "25ch" }}
              defaultValue={selectionHotkey}
            />
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
  padding: 10px;

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