import React, { useImperativeHandle } from "react"
import styled from "styled-components"

export type WoxSettingThemesRefHandler = {}

export type WoxSettingThemesProps = {}

export default React.forwardRef((_props: WoxSettingThemesProps, ref: React.Ref<WoxSettingThemesRefHandler>) => {
  useImperativeHandle(ref, () => ({}))
  return <Style>Themes</Style>
})

const Style = styled.div`
  padding: 10px;
`