import React, { useImperativeHandle } from "react"
import styled from "styled-components"
import WoxThemePreview from "../themes/WoxThemePreview.tsx"

export type WoxSettingThemesRefHandler = {}

export type WoxSettingThemesProps = {}

export default React.forwardRef((_props: WoxSettingThemesProps, ref: React.Ref<WoxSettingThemesRefHandler>) => {
  useImperativeHandle(ref, () => ({}))
  return (
    <Style>
      <WoxThemePreview />
    </Style>
  )
})

const Style = styled.div`
  margin-top: -32px;
`
