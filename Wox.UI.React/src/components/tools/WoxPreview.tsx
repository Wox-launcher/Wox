import { WOXMESSAGE } from "../../entity/WoxMessage.typings"
import WoxScrollbar from "./WoxScrollbar.tsx"
import { WoxPreviewTypeEnum } from "../../enums/WoxPreviewTypeEnum.ts"
import Markdown from "react-markdown"
import styled from "styled-components"
import { WoxThemeHelper } from "../../utils/WoxThemeHelper.ts"
import { Theme } from "../../entity/Theme.typings"

export default (props: { preview: WOXMESSAGE.WoxPreview; resultSingleItemHeight: number }) => {
  return (
    <Style theme={WoxThemeHelper.getInstance().getTheme()}>
      <WoxScrollbar scrollbarProps={{ autoHeight: true, autoHeightMin: 0, autoHeightMax: props.resultSingleItemHeight * 8 + 10 }}>
        <div className={"wox-query-result-preview-content"}>
          {props.preview.PreviewType === WoxPreviewTypeEnum.WoxPreviewTypeText.code && <p className={"wox-query-result-preview-text"}>{props.preview.PreviewData}</p>}
          {props.preview.PreviewType === WoxPreviewTypeEnum.WoxPreviewTypeImage.code && <img className={"wox-query-result-preview-image"} src={props.preview.PreviewData} />}
          {props.preview.PreviewType === WoxPreviewTypeEnum.WoxPreviewTypeMarkdown.code && <Markdown>{props.preview.PreviewData}</Markdown>}
          {props.preview.PreviewType === WoxPreviewTypeEnum.WoxPreviewTypeUrl.code && <iframe className={"wox-query-result-preview-url"} src={props.preview.PreviewData}></iframe>}
        </div>
      </WoxScrollbar>

      {Object.keys(props.preview.PreviewProperties)?.length > 0 && (
        <div className={"wox-query-result-preview-properties"}>
          {Object.keys(props.preview.PreviewProperties)?.map(key => {
            return (
              <div key={`key-${key}`} className={"wox-query-result-preview-property"}>
                <div className={"wox-query-result-preview-property-key"}>{key}</div>
                <div className={"wox-query-result-preview-property-value"}>{props.preview.PreviewProperties[key]}</div>
              </div>
            )
          })}
        </div>
      )}
    </Style>
  )
}

const Style = styled.div<{ theme: Theme }>`
  flex: 1;
  position: relative;
  box-sizing: border-box;
  width: 50%;
  display: inline-block;
  border-left: 1px solid ${props => props.theme.PreviewSplitLineColor};
  padding: 10px 0 10px 10px;
  color: ${props => props.theme.PreviewFontColor};

  .wox-query-result-preview-content {
    overflow: hidden;
    position: relative;

    .wox-query-result-preview-text {
      word-wrap: break-word;
      white-space: pre-line;
      margin-top: 0;
    }

    .wox-query-result-preview-image {
      width: 100%;
      max-height: 400px;
    }

    .wox-query-result-preview-url {
      width: 100%;
      height: 400px;
    }
  }

  .wox-query-result-preview-properties {
    position: absolute;
    left: 0;
    bottom: 0;
    right: 0;
    overflow: hidden;

    .wox-query-result-preview-property {
      display: flex;
      justify-content: space-between;
      width: 100%;
      border-top: 1px solid ${props => props.theme.PreviewSplitLineColor};
      padding: 2px 10px;
      overflow: hidden;

      .wox-query-result-preview-property-key {
        font-weight: 500;
        color: ${props => props.theme.PreviewPropertyTitleColor};
      }

      .wox-query-result-preview-property-value {
        color: ${props => props.theme.PreviewPropertyContentColor};
      }

      .div {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
    }
  }
`
