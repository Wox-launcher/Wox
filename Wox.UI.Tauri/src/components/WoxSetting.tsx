import React, { useState } from "react"
import { Box, Tab, Tabs } from "@mui/material"
import ExtensionIcon from "@mui/icons-material/Extension"
import SettingsIcon from "@mui/icons-material/Settings"
import DarkModeIcon from "@mui/icons-material/DarkMode"
import styled from "styled-components"
import WoxSettingGeneral from "./settings/WoxSettingGeneral.tsx"
import WoxSettingPlugins from "./settings/WoxSettingPlugins.tsx"
import WoxSettingThemes from "./settings/WoxSettingThemes.tsx"

export default () => {
  const [tabIndex, setTabIndex] = useState(0)

  const handleTabChange = (_event: React.SyntheticEvent, newValue: number) => {
    setTabIndex(newValue)
  }

  return <Style>
    <Box sx={{ flexGrow: 1, display: "flex", height: 800 }}>
      <Tabs value={tabIndex} onChange={handleTabChange} orientation="vertical">
        <Tab icon={<SettingsIcon />} label="General" sx={{
          textTransform: "none"
        }} />
        <Tab icon={<ExtensionIcon />} label="Plugins" sx={{
          textTransform: "none"
        }} />
        <Tab icon={<DarkModeIcon />} label="Themes" sx={{
          textTransform: "none"
        }} />
      </Tabs>
      <div className={"setting-container"}>
        {tabIndex == 0 && <WoxSettingGeneral />}
        {tabIndex == 1 && <WoxSettingPlugins />}
        {tabIndex == 2 && <WoxSettingThemes />}
      </div>

    </Box>
  </Style>
}

const Style = styled.div`
  .MuiTabs-root {
    background-color: #2e1534;

    .MuiTab-labelIcon {
      color: #ffffff !important;
    }

    .Mui-selected {
      color: cornflowerblue !important;
    }
  }

  .setting-container {
    width: 100%;
    height: 100%;
    background-color: black;
  }
`