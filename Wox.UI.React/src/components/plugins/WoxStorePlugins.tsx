import React, { useEffect, useState } from "react"
import { WoxPluginHelper } from "../../utils/WoxPluginHelper.ts"
import { CircularProgress, Divider, List, ListItem, ListItemAvatar, ListItemText, Typography } from "@mui/material"
import styled from "styled-components"
import { StorePluginManifest } from "../../entity/Plugin.typing"

export default () => {
  const [loading, setLoading] = useState(true)
  const [plugins, setPlugins] = useState<StorePluginManifest[]>([])

  useEffect(() => {
    WoxPluginHelper.getInstance()
      .loadStorePlugins()
      .then(_ => {
        setPlugins([...WoxPluginHelper.getInstance().getStorePlugins(), ...WoxPluginHelper.getInstance().getStorePlugins()])
        setLoading(false)
      })
  }, [])
  return (
    <Style>
      {loading && <CircularProgress />}
      {!loading && (
        <List sx={{ width: "100%" }}>
          {plugins.map((plugin, index) => {
            return (
              <div key={`list-item-${index}`}>
                <ListItem>
                  <ListItemAvatar>
                    <img alt={plugin.Name} src={plugin.IconUrl} height={36} width={36} />
                  </ListItemAvatar>
                  <ListItemText
                    primary={plugin.Name}
                    secondary={
                      <React.Fragment>
                        <Typography sx={{ display: "inline", color: "white" }} component="span" variant="body2" color="text.primary">
                          {plugin.Author}
                        </Typography>
                        <span className={"plugin-description"}>{` — ${plugin.Description}`}</span>
                      </React.Fragment>
                    }
                  />
                </ListItem>
                <Divider variant="inset" component="li" sx={{ borderColor: "#23272d" }} />
              </div>
            )
          })}
        </List>
      )}
    </Style>
  )
}

const Style = styled.div`
  .plugin-description {
    color: #787b8b;
  }
`
