import React, { useImperativeHandle } from "react"
import styled from "styled-components"

export type WoxSettingPluginsRefHandler = {}

export type WoxSettingPluginsProps = {}

export default React.forwardRef((_props: WoxSettingPluginsProps, ref: React.Ref<WoxSettingPluginsRefHandler>) => {
  useImperativeHandle(ref, () => ({}))
  return <Style>Plugins</Style>
})

const Style = styled.div`
  padding: 10px;
`