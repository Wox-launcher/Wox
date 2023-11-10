import React, { useImperativeHandle } from "react"
import styled from "styled-components"
import { Theme } from "../../entity/Theme.typings"
import { WoxThemeHelper } from "../../utils/WoxThemeHelper.ts"

export type WoxSettingGeneralRefHandler = {}

export type WoxSettingGeneralProps = {}

export default React.forwardRef((_props: WoxSettingGeneralProps, ref: React.Ref<WoxSettingGeneralRefHandler>) => {
  useImperativeHandle(ref, () => ({}))
  return <Style theme={WoxThemeHelper.getInstance().getTheme()}>General</Style>
})

const Style = styled.div<{ theme: Theme }>`
  padding: 10px;
`