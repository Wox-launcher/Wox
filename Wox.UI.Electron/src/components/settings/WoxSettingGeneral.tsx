import React, { useImperativeHandle } from "react"
import styled from "styled-components"
import { Checkbox, TextField } from "@mui/material"

export type WoxSettingGeneralRefHandler = {}

export type WoxSettingGeneralProps = {}

export default React.forwardRef((_props: WoxSettingGeneralProps, ref: React.Ref<WoxSettingGeneralRefHandler>) => {
  useImperativeHandle(ref, () => ({}))
  return <Style>
    <div className={"wox-setting-item"}>
      <div className={"setting-item-label"}>Startup:</div>
      <div className={"setting-item-content"}>
        <div className={"setting-item-detail"}><Checkbox /> Launch Wox On Startup</div>
        <div className={"setting-item-intro"}>If selected, Wox will launch on startup</div>
      </div>
      <div className={"wox-setting-additional"}></div>
    </div>
    <div className={"wox-setting-item"}>
      <div className={"setting-item-label"}>Wox Main Hotkey:</div>
      <div className={"setting-item-content"}>
        <div className={"setting-item-detail"}>
          <TextField
            label="Main Hotkey"
            InputLabelProps={{
              shrink: true
            }}
            focused
            sx={{ m: 1, width: "25ch" }}
          />
        </div>
      </div>
    </div>
    <div className={"wox-setting-item"}>
      <div className={"setting-item-label"}>Wox Selection Hotkey:</div>
      <div className={"setting-item-content"}>
        <div className={"setting-item-detail"}>
          <TextField
            label="Selection Hotkey"
            InputLabelProps={{
              shrink: true
            }}
            focused
            sx={{ m: 1, width: "25ch" }}
          />
        </div>
      </div>
    </div>
    <div className={"wox-setting-item"}>
      <div className={"setting-item-label"}>Use PinYin:</div>
      <div className={"setting-item-content"}>
        <div className={"setting-item-detail"}><Checkbox /> Use PinYin For Searching</div>
        <div className={"setting-item-intro"}>If selected, When searching, it converts Chinese into Pinyin and matches it.</div>
      </div>
    </div>
    <div className={"wox-setting-item"}>
      <div className={"setting-item-label"}>Hide On Lost Focus:</div>
      <div className={"setting-item-content"}>
        <div className={"setting-item-detail"}><Checkbox /> Hide Wox On Lost Focus</div>
        <div className={"setting-item-intro"}>If selected, When wox lost focus, it will be hidden.</div>
      </div>
    </div>
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

      .MuiCheckbox-root {
        color: #1976d2;
      }
    }
  }
`