import React, { useEffect, useState } from "react"
import { WoxPluginHelper } from "../../utils/WoxPluginHelper.ts"
import { Box, Button, CircularProgress, Divider, List, ListItem, ListItemAvatar, ListItemText } from "@mui/material"
import styled from "styled-components"
import { StorePluginManifest } from "../../entity/Plugin.typing"
import WoxScrollbar from "../tools/WoxScrollbar.tsx"
import { useWindowSize } from "usehooks-ts"

export default () => {
  const [loading, setLoading] = useState(true)
  const [plugins, setPlugins] = useState<StorePluginManifest[]>([])

  const size = useWindowSize()

  useEffect(() => {
    WoxPluginHelper.getInstance()
      .loadStorePlugins()
      .then(_ => {
        const plugins = []
        for (let i = 0; i < 10; i++) {
          plugins.push(WoxPluginHelper.getInstance().getStorePlugins()[0])
        }
        setPlugins(plugins)
        setLoading(false)
      })
  }, [])

  return (
    <Style>
      {loading && <CircularProgress />}
      {!loading && (
        <Box sx={{ flexGrow: 1, display: "flex", height: "100%" }}>
          <WoxScrollbar
            className={"plugin-list-scrollbars"}
            scrollbarProps={{
              style: { width: "50%" },
              autoHeightMax: size.height - 80
            }}
          >
            <List sx={{ width: "100%" }}>
              {plugins.map((plugin, index) => {
                return (
                  <div key={`list-item-${index}`}>
                    <ListItem>
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
                    </ListItem>
                    <Divider variant="inset" component="li" sx={{ borderColor: "#23272d" }} />
                  </div>
                )
              })}
            </List>
          </WoxScrollbar>
        </Box>
      )}
    </Style>
  )
}

const Style = styled.div`
  .plugin-list-scrollbars {
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
`
