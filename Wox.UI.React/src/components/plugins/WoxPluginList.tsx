import WoxScrollbar from "../tools/WoxScrollbar.tsx"
import { Box, Button, CircularProgress, Divider, List, ListItemAvatar, ListItemButton, ListItemText, Tab, Tabs, Typography } from "@mui/material"
import ImageGallery from "react-image-gallery"
import { useWindowSize } from "usehooks-ts"
import { InstalledPluginManifest, StorePluginManifest } from "../../entity/Plugin.typing"
import React, { useState } from "react"
import styled from "styled-components"
import { installPlugin, openUrl } from "../../api/WoxAPI.ts"
import WoxImage from "../tools/WoxImage.tsx"

export default (props: { plugins: StorePluginManifest[] | InstalledPluginManifest[]; type: string; refreshCallback?: () => void }) => {
  const { plugins } = props
  const [installLoading, setInstallLoading] = useState(false)
  const [selectedTab, setSelectedTab] = useState(0)
  const [selectedIndex, setSelectedIndex] = useState(0)
  const size = useWindowSize()
  const isStore = props.type === "store"

  const handleChange = (_event: React.SyntheticEvent, newValue: number) => {
    setSelectedTab(newValue)
  }

  return (
    <Style>
      <Box sx={{ flexGrow: 1, display: "flex", height: "calc(100vh - 85px)" }}>
        <div className={"plugin-list-container"}>
          <WoxScrollbar
            className={"plugin-list-scrollbars"}
            scrollbarProps={{
              autoHeightMax: size.height - 80
            }}
          >
            <List sx={{ width: "100%" }}>
              {plugins.map((plugin, index) => {
                return (
                  <div key={`list-item-${index}`}>
                    <ListItemButton
                      selected={index === selectedIndex}
                      onClick={() => {
                        setSelectedIndex(index)
                      }}
                    >
                      <ListItemAvatar>
                        <WoxImage img={plugin.Icon} height={36} width={36} />
                      </ListItemAvatar>
                      <ListItemText primary={plugin.Name} secondary={<span className={"plugin-description"}>{plugin.Description}</span>} />
                      {isStore && !plugin.IsInstalled && (
                        <>
                          {installLoading && <CircularProgress size={24} sx={{ marginLeft: "5px" }} />}
                          {!installLoading && (
                            <Button
                              variant="contained"
                              sx={{ textTransform: "none", marginLeft: "5px" }}
                              onClick={event => {
                                setInstallLoading(true)
                                installPlugin(plugin.Id).then(resp => {
                                  if (!resp.Success) {
                                    alert(resp.Message)
                                  }
                                  setInstallLoading(false)
                                })
                                event.preventDefault()
                                event.stopPropagation()
                              }}
                            >
                              Install
                            </Button>
                          )}
                        </>
                      )}
                      {plugin.NeedUpdate && (
                        <Button variant="contained" sx={{ textTransform: "none", marginLeft: "5px" }}>
                          Update
                        </Button>
                      )}
                    </ListItemButton>
                    <Divider variant="inset" component="li" sx={{ borderColor: "#23272d" }} />
                  </div>
                )
              })}
            </List>
          </WoxScrollbar>
        </div>
        {plugins && plugins.length > 0 && (
          <div className={"plugin-detail-container"}>
            <div className={"plugin-detail-summary"}>
              <div className={"detail-title"}>
                <Typography variant="h4" gutterBottom display={"inline"}>
                  {plugins[selectedIndex].Name}
                </Typography>
                <Typography variant="subtitle2" display={"inline"} gutterBottom sx={{ paddingLeft: "10px" }}>
                  Version: {plugins[selectedIndex].Version}
                </Typography>
              </div>

              <div className={"detail-subtitle"}>
                <Typography variant="subtitle1" sx={{ color: "#6f737a" }} display={"inline"} gutterBottom>
                  {plugins[selectedIndex].Author} -{" "}
                </Typography>
                <Typography
                  onClick={async () => {
                    await openUrl(plugins[selectedIndex].Website)
                  }}
                  variant="subtitle1"
                  display={"inline"}
                  sx={{ color: "#6a99f6", cursor: "pointer" }}
                  gutterBottom
                >
                  Plugin homepage
                </Typography>
              </div>
            </div>

            <Tabs value={selectedTab} sx={{ borderBottom: "1px solid #23272d" }} onChange={handleChange}>
              <Tab label="Description" sx={{ textTransform: "none", color: "white" }} />
              {!isStore && <Tab label="Setting" sx={{ textTransform: "none", color: "white" }} />}
            </Tabs>

            <div className={"plugin-detail-description"} style={{ display: `${selectedTab === 0 ? "block" : "none"}` }}>
              {isStore && plugins[selectedIndex].ScreenshotUrls && (
                <ImageGallery
                  showNav={false}
                  showThumbnails={false}
                  showFullscreenButton={false}
                  showPlayButton={false}
                  items={
                    plugins[selectedIndex].ScreenshotUrls?.map(value => {
                      return { original: value, thumbnail: value }
                    }) || []
                  }
                />
              )}
              <Typography variant="body1" gutterBottom>
                {plugins[selectedIndex].Description}
              </Typography>
            </div>
          </div>
        )}
      </Box>
    </Style>
  )
}

const Style = styled.div`
  .plugin-list-container {
    height: 100%;
    flex: 1;
    border-right: 1px solid #23272d;
  }
  .plugin-detail-container {
    flex: 1;
  }

  .plugin-description {
    color: #787b8b;
    display: inline-block;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    width: calc(50vw - 280px);
  }

  .plugin-detail-container {
    width: 100%;
  }

  .plugin-detail-summary {
    padding: 15px;
  }

  .plugin-detail-description {
    padding: 15px;

    .image-gallery-content .image-gallery-slide .image-gallery-image {
      max-height: 350px;
    }
  }
`
