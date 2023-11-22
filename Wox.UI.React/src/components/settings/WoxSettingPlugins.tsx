import React, { useImperativeHandle } from "react"
import styled from "styled-components"
import { Box, Tab, Tabs } from "@mui/material"
import WoxInstalledPlugins from "../plugins/WoxInstalledPlugins.tsx"
import WoxStorePlugins from "../plugins/WoxStorePlugins.tsx"

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
          <Tab tabIndex={0} label="Installed" sx={tabSxProps} />
          <Tab tabIndex={1} label="Store" sx={tabSxProps} />
        </Tabs>
        <div className={"plugin-container"} style={{ display: selectedTab === 0 ? "block" : "none" }}>
          <WoxInstalledPlugins />
        </div>
        <div className={"plugin-container"} style={{ display: selectedTab === 1 ? "block" : "none" }}>
          <WoxStorePlugins />
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
