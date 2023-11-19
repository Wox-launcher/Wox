import React, {useImperativeHandle} from "react"
import styled from "styled-components"
import {Theme} from "../entity/Theme.typings"
import {WoxThemeHelper} from "../utils/WoxThemeHelper.ts"
import {WOXMESSAGE} from "../entity/WoxMessage.typings"
import {WoxQueryTypeEnum} from "../enums/WoxQueryTypeEnum.ts"

export type WoxQueryBoxRefHandler = {
    changeQuery: (changedQuery: WOXMESSAGE.ChangedQuery) => void
    selectAll: () => void
    focus: () => void
    getQuery: () => string
}

export type WoxQueryBoxProps = {
    defaultValue?: string
    onQueryChange: (changedQuery: WOXMESSAGE.ChangedQuery) => void
    onFocus?: () => void
    onClick?: () => void
}

export default React.forwardRef((_props: WoxQueryBoxProps, ref: React.Ref<WoxQueryBoxRefHandler>) => {
    const queryBoxRef = React.createRef<HTMLInputElement>()

    const selectInputText = () => {
        queryBoxRef.current?.select()
    }

    useImperativeHandle(ref, () => ({
        changeQuery: (changedQuery: WOXMESSAGE.ChangedQuery) => {
            if (queryBoxRef.current) {
                if (changedQuery.QueryType === WoxQueryTypeEnum.WoxQueryTypeInput.code) {
                    queryBoxRef.current!.value = changedQuery.QueryText
                }
                if (changedQuery.QueryType === WoxQueryTypeEnum.WoxQueryTypeSelection.code) {
                    queryBoxRef.current!.value = ""
                }
                _props.onQueryChange(changedQuery)
            }
        },
        selectAll: () => {
            selectInputText()
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
               onClick={() => {
                   _props.onClick?.()
               }}
               onChange={(e) => {
                   _props.onQueryChange({
                       QueryText: e.target.value,
                       QueryType: WoxQueryTypeEnum.WoxQueryTypeInput.code
                   } as WOXMESSAGE.ChangedQuery)
               }}

        />
        <div className={"dragging-container"}>&nbsp;</div>
    </Style>
})

const Style = styled.div<{ theme: Theme }>`
  position: relative;
  width: 100%;
  overflow: hidden;

  input {
    height: 50px;
    line-height: 50px;
    width: 100%;
    font-size: 24px;
    outline: none;
    padding-left: 10px;
    border: 0;
    cursor: auto;
    color: ${props => props.theme.QueryBoxFontColor};
    background-color: ${props => props.theme.QueryBoxBackgroundColor};
    border-radius: ${props => props.theme.QueryBoxBorderRadius}px;
    display: inline-block;
    box-sizing: border-box;
  }

  .dragging-container {
    position: absolute;
    top: 0;
    right: 0;
    bottom: 0;
    background-color: transparent;
    width: 120px;
    z-index: 999;
    -webkit-app-region: drag;
  }
`