import React, { useEffect, useImperativeHandle, useState } from "react"
import styled from "styled-components"
import WoxThemePreview from "../themes/WoxThemePreview.tsx"
import { WoxThemeHelper } from "../../utils/WoxThemeHelper.ts"
import { Box, Card, CardContent, Chip, Divider, Skeleton, Typography } from "@mui/material"
import { Theme } from "../../entity/Theme.typings"

export type WoxSettingThemesRefHandler = {}

export type WoxSettingThemesProps = {}

export default React.forwardRef((_props: WoxSettingThemesProps, ref: React.Ref<WoxSettingThemesRefHandler>) => {
  const [loading, setLoading] = useState(true)
  const [selectedThemeIndex, setSelectedThemeIndex] = useState(0)
  const [currentTheme, setCurrentTheme] = useState<Theme>({} as Theme)
  const [themes, setThemes] = useState<Theme[]>([])
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
        <Card sx={{ width: "100%", borderRadius: 0, background: "transparent", boxShadow: "initial" }}>
          <Typography variant="h6" component="div" sx={{ padding: "5px 16px", color: "white" }}>
            Available Themes
          </Typography>
          <Divider />
          <CardContent>
            {themes.map((item, index) => {
              return index === selectedThemeIndex ? (
                <Chip
                  label={item.ThemeName}
                  color={"primary"}
                  onClick={() => {
                    setSelectedThemeIndex(index)
                    setCurrentTheme(themes[index])
                  }}
                />
              ) : (
                <Chip
                  label={item.ThemeName}
                  onClick={() => {
                    setSelectedThemeIndex(index)
                    setCurrentTheme(themes[index])
                  }}
                />
              )
            })}
          </CardContent>
        </Card>
      )}
    </Style>
  )
})

const Style = styled.div`
  margin-top: -32px;
`
