import React, { useEffect, useImperativeHandle, useState } from "react"
import styled from "styled-components"
import WoxThemePreview from "../themes/WoxThemePreview.tsx"
import { WoxThemeHelper } from "../../utils/WoxThemeHelper.ts"
import { Box, List, ListItemButton, ListItemText, Skeleton } from "@mui/material"
import { Theme } from "../../entity/Theme.typings"
import { useWindowSize } from "usehooks-ts"
import WoxScrollbar from "../tools/WoxScrollbar.tsx"

export type WoxSettingThemesRefHandler = {}

export type WoxSettingThemesProps = {}

export default React.forwardRef((_props: WoxSettingThemesProps, ref: React.Ref<WoxSettingThemesRefHandler>) => {
  const [loading, setLoading] = useState(true)
  const [selectedThemeIndex, setSelectedThemeIndex] = useState(0)
  const [currentTheme, setCurrentTheme] = useState<Theme>({} as Theme)
  const [themes, setThemes] = useState<Theme[]>([])
  const size = useWindowSize()

  useImperativeHandle(ref, () => ({}))

  useEffect(() => {
    WoxThemeHelper.getInstance()
      .loadStoreThemes()
      .then(() => {
        const currentStoreThemes = WoxThemeHelper.getInstance().getStoreThemes()
        setThemes(currentStoreThemes)
        setCurrentTheme(currentStoreThemes[0])
        setLoading(false)
      })
  }, [])

  return (
    <Style>
      <WoxThemePreview theme={currentTheme} />
      {loading && (
        <Box sx={{ width: "100%", padding: "16px" }}>
          <Skeleton />
          <Skeleton animation="wave" />
          <Skeleton animation={false} />
        </Box>
      )}
      {!loading && (
        <Box sx={{ display: "flex", flexGrow: 1, height: size.height - 400 }}>
          <div className={"theme-list-container"}>
            <WoxScrollbar
              className={"theme-list-scrollbars"}
              scrollbarProps={{
                autoHeightMax: size.height - 400
              }}
            >
              <List>
                {themes.map((item, index) => {
                  return (
                    <ListItemButton
                      selected={index === selectedThemeIndex}
                      onClick={() => {
                        setSelectedThemeIndex(index)
                      }}
                    >
                      <ListItemText primary={item.ThemeName} secondary={<span className={"theme-author"}>{item.ThemeAuthor}</span>} />
                    </ListItemButton>
                  )
                })}
              </List>
            </WoxScrollbar>
          </div>
          <div className={"theme-setting-container"}>&nbsp;</div>
        </Box>
        // <Card sx={{ width: "100%", borderRadius: 0, background: "transparent", boxShadow: "initial" }}>
        //   <Typography variant="h6" component="div" sx={{ padding: "5px 16px", color: "white" }}>
        //     Available Themes
        //   </Typography>
        //   <Divider />
        //   <CardContent>
        //     {themes.map((item, index) => {
        //       return index === selectedThemeIndex ? (
        //         <Chip
        //           label={item.ThemeName}
        //           color={"primary"}
        //           onClick={() => {
        //             setSelectedThemeIndex(index)
        //             setCurrentTheme(themes[index])
        //           }}
        //         />
        //       ) : (
        //         <Chip
        //           label={item.ThemeName}
        //           onClick={() => {
        //             setSelectedThemeIndex(index)
        //             setCurrentTheme(themes[index])
        //           }}
        //         />
        //       )
        //     })}
        //   </CardContent>
        // </Card>
      )}
    </Style>
  )
})

const Style = styled.div`
  margin-top: -32px;
  .theme-list-container,
  .theme-setting-container {
    flex: 1;
    height: 100%;
  }

  .theme-list-container {
    border-right: 1px solid #23272d;
  }

  .theme-author {
    color: #787b8b;
    display: inline-block;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    width: calc(50vw - 280px);
  }
`
