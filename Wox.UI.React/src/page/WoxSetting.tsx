import React, { useEffect, useState } from "react"
import { Box, Tab, Tabs } from "@mui/material"
import ExtensionIcon from "@mui/icons-material/Extension"
import SettingsIcon from "@mui/icons-material/Settings"
import DarkModeIcon from "@mui/icons-material/DarkMode"
import styled from "styled-components"
import WoxSettingGeneral, { WoxSettingGeneralRefHandler } from "../components/settings/WoxSettingGeneral.tsx"
import WoxSettingPlugins from "../components/settings/WoxSettingPlugins.tsx"
import WoxSettingThemes from "../components/settings/WoxSettingThemes.tsx"

import { WoxSettingHelper } from "../utils/WoxSettingHelper.ts"

export default () => {
  const [tabIndex, setTabIndex] = useState(0)
  const tabSxSetting = { textTransform: "none" }
  const woxSettingGeneralRef = React.useRef<WoxSettingGeneralRefHandler>(null)

  const handleTabChange = (_event: React.SyntheticEvent, newValue: number) => {
    setTabIndex(newValue)
  }

  useEffect(() => {
    WoxSettingHelper.getInstance().loadSetting().then(_ => {
      woxSettingGeneralRef.current?.initialize()
    })
  }, [])
  return <Style>
    <Box sx={{ flexGrow: 1, display: "flex", height: 800 }}>
      <Tabs value={tabIndex} onChange={handleTabChange} orientation="vertical">
        <Tab icon={<SettingsIcon />} label="General" sx={tabSxSetting} />
        <Tab icon={<ExtensionIcon />} label="Plugins" sx={tabSxSetting} />
        <Tab icon={<DarkModeIcon />} label="Themes" sx={tabSxSetting} />
      </Tabs>
      <div className={"setting-container"}>
        <div className={"setting-item"} style={{ display: tabIndex === 0 ? "block" : "none" }}><WoxSettingGeneral ref={woxSettingGeneralRef} /></div>
        <div className={"setting-item"} style={{ display: tabIndex === 1 ? "block" : "none" }}><WoxSettingPlugins /></div>
        <div className={"setting-item"} style={{ display: tabIndex === 2 ? "block" : "none" }}><WoxSettingThemes /></div>
      </div>

    </Box>
  </Style>
}

const Style = styled.div`
  .MuiTabs-root {
    background-color: rgba(52, 57, 60, 0.98);

    .MuiTab-labelIcon {
      color: white !important;
    }

    .Mui-selected {
      color: #1976d2 !important;
    }
  }

  .setting-container {
    width: 100%;
    height: 100%;
    background-color: rgba(44, 48, 50, 0.7);
  }
`