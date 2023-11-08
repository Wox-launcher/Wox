import React, { useImperativeHandle } from "react"
import styled from "styled-components"
import { WoxTauriHelper } from "../utils/WoxTauriHelper.ts"
import { getTheme } from "../api/WoxAPI.ts"
import { Theme } from "../entity/Theme.typings"
import { WoxThemeHelper } from "../utils/WoxThemeHelper.ts"

export type WoxQueryBoxRefHandler = {
  changeQuery: (query: string) => void
  selectAll: () => void
  focus: () => void
  getQuery: () => string
}

export type WoxQueryBoxProps = {
  defaultValue?: string
  onQueryChange: (query: string) => void
  onFocus?: () => void
}

export default React.forwardRef((_props: WoxQueryBoxProps, ref: React.Ref<WoxQueryBoxRefHandler>) => {
  const queryBoxRef = React.createRef<HTMLInputElement>()

  useImperativeHandle(ref, () => ({
    changeQuery: (query: string) => {
      if (queryBoxRef.current) {
        queryBoxRef.current!.value = query
        _props.onQueryChange(query)
      }
    },
    selectAll: () => {
      queryBoxRef.current?.select()
    },
    focus: () => {
      queryBoxRef.current?.focus()
    },
    getQuery: () => {
      return queryBoxRef.current?.value ?? ""
    }
  }))

  return <Style theme={WoxThemeHelper.getInstance().getTheme()} className="wox-query-box">
    <input ref={queryBoxRef}
           title={"Query Wox"}
           className={"mousetrap"}
           type="text"
           aria-label="Wox"
           autoComplete="off"
           autoCorrect="off"
           autoFocus={true}
           autoCapitalize="off"
           defaultValue={_props.defaultValue}
           onFocus={() => {
             _props.onFocus?.()
           }}
           onChange={(e) => {
             _props.onQueryChange(e.target.value)
           }} />
    <button className={"wox-setting"} onClick={() => {
      getTheme().then(resp => console.log(resp.data))
    }} onMouseMoveCapture={(event) => {
      WoxTauriHelper.getInstance().startDragging().then(_ => {
        queryBoxRef.current?.focus()
      })
      event.preventDefault()
      event.stopPropagation()
    }}>Wox
    </button>
  </Style>
})

const Style = styled.div<{ theme: Theme }>`
  position: relative;
  width: ${WoxTauriHelper.getInstance().getWoxWindowWidth()}px;
  overflow: hidden;

  input {
    height: 60px;
    line-height: 60px;
    width: ${WoxTauriHelper.getInstance().getWoxWindowWidth()}px;
    font-size: 24px;
    outline: none;
    padding: 10px;
    border: 0;
    background-color: transparent;
    cursor: auto;
    color:  ${props => props.theme.QueryBoxFontColor};
    display: inline-block;
    box-sizing: border-box;
  }

  .wox-placeholder {
    position: absolute;
    left: 10px;
    top: ${WoxTauriHelper.getInstance().isTauri() ? "12px" : "11px"};
    font-size: 24px;
    color: #545454;
  }

  .wox-setting {
    position: absolute;
    bottom: 3px;
    right: 4px;
    top: 3px;
    padding: 0 10px;
    background: transparent;
    border: none;
    color: #545454;
  }
`