import React, { useImperativeHandle } from "react"
import styled from "styled-components"

export type WoxQueryBoxRefHandler = {
  changeQuery: (query: string) => void
  selectAll: () => void
}

export type WoxQueryBoxProps = {
  defaultValue?: string
  onQueryChange: (query: string) => void
}

export default React.forwardRef((_props: WoxQueryBoxProps, ref: React.Ref<WoxQueryBoxRefHandler>) => {
  const queryBoxRef = React.createRef<
    HTMLInputElement
  >()

  useImperativeHandle(ref, () => ({
    changeQuery: (query: string) => {
      if (queryBoxRef.current) {
        queryBoxRef.current!.value = query
        _props.onQueryChange(query)
      }
    },
    selectAll: () => {
      queryBoxRef.current?.select()
    }
  }))

  return <Style className="wox-query-box">
    <input ref={queryBoxRef} title={"Query Wox"}
           type="text"
           aria-label="Wox"
           autoComplete="off"
           autoCorrect="off"
           autoCapitalize="off"
           defaultValue={_props.defaultValue}
           onChange={(e) => {
             _props.onQueryChange(e.target.value)
           }} />
    <button className={"wox-setting"}>Wox</button>
  </Style>
})

const Style = styled.div`
  position: relative;
  width: 100%;

  input {
    height: 60px;
    line-height: 60px;
    width: 800px;
    font-size: 24px;
    outline: none;
    padding: 10px;
    border: 0;
    background-color: transparent;
    -webkit-app-region: no-drag;
    cursor: auto;
    display: inline-block;
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