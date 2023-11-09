import React, { useImperativeHandle } from "react"
import styled from "styled-components"

export type WoxSettingGeneralRefHandler = {}

export type WoxSettingGeneralProps = {}

export default React.forwardRef((_props: WoxSettingGeneralProps, ref: React.Ref<WoxSettingGeneralRefHandler>) => {
  useImperativeHandle(ref, () => ({}))
  return <Style>General</Style>
})

const Style = styled.div`
  padding: 10px;
`