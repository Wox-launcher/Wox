import WoxScrollbar from "../tools/WoxScrollbar.tsx"
import { Box, Button, Divider, List, ListItemAvatar, ListItemButton, ListItemText, Tab, Tabs, Typography } from "@mui/material"
import ImageGallery from "react-image-gallery"
import { useWindowSize } from "usehooks-ts"
import { StorePluginManifest } from "../../entity/Plugin.typing"
import { useState } from "react"
import styled from "styled-components"

export default (props: { plugins: StorePluginManifest[]; type: string }) => {
  const { plugins } = props
  const [selectedIndex, setSelectedIndex] = useState(0)
  const size = useWindowSize()

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
                        <img alt={plugin.Name} src={plugin.IconUrl} height={36} width={36} />
                      </ListItemAvatar>
                      <ListItemText primary={plugin.Name} secondary={<span className={"plugin-description"}>{plugin.Description}</span>} />
                      {!plugin.IsInstalled && (
                        <Button variant="outlined" sx={{ textTransform: "none", marginLeft: "5px" }}>
                          Install
                        </Button>
                      )}
                      {plugin.NeedUpdate && (
                        <Button variant="outlined" sx={{ textTransform: "none", marginLeft: "5px" }}>
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
        <div className={"plugin-detail-container"}>
          <div className={"plugin-detail-summary"}>
            <Typography variant="h4" gutterBottom>
              {plugins[selectedIndex].Name}
              <Typography variant="subtitle2" display={"inline"} gutterBottom sx={{ paddingLeft: "10px" }}>
                Version: {plugins[selectedIndex].Version}
              </Typography>
            </Typography>
            <Typography variant="subtitle1" sx={{ color: "#6f737a" }} display={"inline"} gutterBottom>
              {plugins[selectedIndex].Author} -{" "}
            </Typography>
            <Typography
              onClick={() => {
                window.open(plugins[selectedIndex].Website)
              }}
              variant="subtitle1"
              display={"inline"}
              sx={{ color: "#6a99f6" }}
              gutterBottom
            >
              Plugin homepage
            </Typography>
          </div>

          <Tabs value={0} sx={{ borderBottom: "1px solid #23272d" }}>
            <Tab label="Description" sx={{ textTransform: "none" }} />
          </Tabs>

          {plugins[selectedIndex].ScreenshotUrls && plugins[selectedIndex].ScreenshotUrls?.length > 0 && (
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

          <Typography variant="body1" gutterBottom sx={{ paddingLeft: "15px", paddingRight: "15px" }}>
            {plugins[selectedIndex].Description}
          </Typography>
        </div>
      </Box>
    </Style>
  )
}

const Style = styled.div`
  .plugin-list-container {
    height: 100%;
    border-right: 1px solid #23272d;
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

  .image-gallery-content .image-gallery-slide .image-gallery-image {
    max-height: 350px;
  }
`
