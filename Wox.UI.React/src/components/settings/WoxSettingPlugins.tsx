import React, { useImperativeHandle } from "react"
import styled from "styled-components"
import { Box, Tab, Tabs } from "@mui/material"
import WoxPluginInstalled from "../plugins/WoxPluginInstalled.tsx"
import WoxPluginStore from "../plugins/WoxPluginStore.tsx"

export type WoxSettingPluginsRefHandler = {}

export type WoxSettingPluginsProps = {}

export default React.forwardRef((_props: WoxSettingPluginsProps, ref: React.Ref<WoxSettingPluginsRefHandler>) => {
  const [selectedTab, setSelectedTab] = React.useState(0)
  const tabSxProps = { textTransform: "none", color: "#787b8b", fontSize: "16px" }
  const handleChange = (_event: React.SyntheticEvent, newValue: number) => {
    setSelectedTab(newValue)
  }
  useImperativeHandle(ref, () => ({}))
  return (
    <Style>
      <Box className={"setting-plugins"} sx={{ width: "100%" }}>
        <Tabs sx={{ borderBottom: "1px solid #23272d" }} value={selectedTab} onChange={handleChange} centered>
          <Tab label="Installed" sx={tabSxProps} />
          <Tab label="Store" sx={tabSxProps} />
        </Tabs>
        <div className={"plugin-container"} style={{ display: selectedTab === 0 ? "block" : "none" }}>
          <WoxPluginInstalled />
        </div>
        <div className={"plugin-container"} style={{ display: selectedTab === 1 ? "block" : "none" }}>
          <WoxPluginStore />
        </div>
      </Box>
    </Style>
  )
})

const Style = styled.div`
  .setting-plugins {
    .Mui-selected {
      color: white;
    }
  }
`
