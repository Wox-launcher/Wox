import React, { useEffect, useState } from "react"
import { Box, Tab, Tabs } from "@mui/material"
import ExtensionIcon from "@mui/icons-material/Extension"
import SettingsIcon from "@mui/icons-material/Settings"
import DarkModeIcon from "@mui/icons-material/DarkMode"
import styled from "styled-components"
import WoxSettingGeneral from "../components/settings/WoxSettingGeneral.tsx"
import WoxSettingPlugins from "../components/settings/WoxSettingPlugins.tsx"
import WoxSettingThemes from "../components/settings/WoxSettingThemes.tsx"
import { WoxThemeHelper } from "../utils/WoxThemeHelper.ts"

export default () => {
  const [tabIndex, setTabIndex] = useState(0)

  const handleTabChange = (_event: React.SyntheticEvent, newValue: number) => {
    setTabIndex(newValue)
  }

  useEffect(() => {
    console.log(JSON.stringify(WoxThemeHelper.getInstance().getTheme()))
  }, [])

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
  // .MuiTabs-root {
    //   background-color: ${props => props.theme.SettingMenuBarBackgroundColor};
  //
  //   .MuiTab-labelIcon {
    //     color: ${props => props.theme.SettingMenuBarBackgroundColor} !important;
  //   }
  //
  //   .Mui-selected {
    //     color: ${props => props.theme.SettingContainerBackgroundColor} !important;
  //   }
  // }

  .setting-container {
    width: 100%;
    height: 100%;
    background-color: ${props => props.theme.SettingContainerBackgroundColor};
  }
`