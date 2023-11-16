import React, { useState } from "react"
import { Box, Tab, Tabs } from "@mui/material"
import ExtensionIcon from "@mui/icons-material/Extension"
import SettingsIcon from "@mui/icons-material/Settings"
import DarkModeIcon from "@mui/icons-material/DarkMode"
import styled from "styled-components"
import WoxSettingGeneral from "../components/settings/WoxSettingGeneral.tsx"
import WoxSettingPlugins from "../components/settings/WoxSettingPlugins.tsx"
import WoxSettingThemes from "../components/settings/WoxSettingThemes.tsx"

export default () => {
  const [tabIndex, setTabIndex] = useState(0)
  const tabSxSetting = { textTransform: "none" }

  const handleTabChange = (_event: React.SyntheticEvent, newValue: number) => {
    setTabIndex(newValue)
  }

  return <Style>
    <Box sx={{ flexGrow: 1, display: "flex", height: 800 }}>
      <Tabs value={tabIndex} onChange={handleTabChange} orientation="vertical">
        <Tab icon={<SettingsIcon />} label="General" sx={tabSxSetting} />
        <Tab icon={<ExtensionIcon />} label="Plugins" sx={tabSxSetting} />
        <Tab icon={<DarkModeIcon />} label="Themes" sx={tabSxSetting} />
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