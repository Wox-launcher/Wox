import styled from "styled-components"
import WoxQueryBox from "../query/WoxQueryBox.tsx"
import { Theme } from "../../entity/Theme.typings"
import { WoxThemeHelper } from "../../utils/WoxThemeHelper.ts"
import WoxQueryResult from "../query/WoxQueryResult.tsx"
import { WOX_QUERY_RESULT_ITEM_HEIGHT } from "../../utils/WoxConst.ts"
import { WOXMESSAGE } from "../../entity/WoxMessage.typings"

export default () => {
  const getInitResultList = () => {
    return [
      {
        QueryId: "641a84d9-e832-4976-9af8-71552e0f7799",
        Id: "cb792d11-dfd4-4899-87df-42d10c18979c",
        Title: "旁白",
        SubTitle: "/System/Library/CoreServices/VoiceOver.app",
        Icon: { ImageType: "url", ImageData: "/voice_over.png" } as WOXMESSAGE.WoxImage,
        Score: 8,
        Preview: {},
        ContextData: "",
        Actions: [
          {
            Id: "",
            Name: "",
            Icon: {
              ImageType: "",
              ImageData: ""
            },
            IsDefault: true,
            PreventHideAfterAction: false
          }
        ],
        RefreshInterval: 0
      },
      {
        QueryId: "641a84d9-e832-4976-9af8-71552e0f7799",
        Id: "cb792d11-dfd4-4899-87df-42d10c18980c",
        Title: "无边记",
        SubTitle: "/System/Applications/Freeform.app",
        Icon: { ImageType: "url", ImageData: "/free_form.png" } as WOXMESSAGE.WoxImage,
        Score: 8,
        Preview: {},
        ContextData: "",
        Actions: [
          {
            Id: "",
            Name: "",
            Icon: {
              ImageType: "",
              ImageData: ""
            },
            IsDefault: true,
            PreventHideAfterAction: false
          }
        ],
        RefreshInterval: 0
      },
      {
        QueryId: "641a84d9-e832-4976-9af8-71552e0f7799",
        Id: "cb792d11-dfd4-4899-87df-42d10c18990c",
        Title: "截屏",
        SubTitle: "/System/Applications/Utilities/Screenshot.app",
        Icon: { ImageType: "url", ImageData: "/screen_shot.png" } as WOXMESSAGE.WoxImage,
        Score: 8,
        Preview: {},
        ContextData: "",
        Actions: [
          {
            Id: "",
            Name: "",
            Icon: {
              ImageType: "",
              ImageData: ""
            },
            IsDefault: true,
            PreventHideAfterAction: false
          }
        ],
        RefreshInterval: 0
      }
    ] as WOXMESSAGE.WoxMessageResponseResult[]
  }

  const getLauncherHeight = () => {
    const theme = WoxThemeHelper.getInstance().getTheme()
    const baseItemHeight = WOX_QUERY_RESULT_ITEM_HEIGHT + theme.ResultItemPaddingTop + theme.ResultItemPaddingBottom
    return 50 + baseItemHeight * 3 + theme.ResultContainerPaddingTop + theme.ResultContainerPaddingBottom + theme.AppPaddingTop + theme.AppPaddingBottom
  }

  return (
    <Style theme={WoxThemeHelper.getInstance().getTheme()} height={getLauncherHeight()}>
      <div className={"wox-preview-launcher"}>
        <div className={"wox-launcher"}>
          <WoxQueryBox defaultValue={"Hello World!"} isPreview={true} />
          <WoxQueryResult isPreview={true} initResultList={getInitResultList()} />
        </div>
      </div>
    </Style>
  )
}

const Style = styled.div<{ theme: Theme; height: number }>`
  .wox-preview-launcher {
    position: relative;
    width: 100%;
    height: 300px;
    background: url("/wox-preview-bg.jpg") no-repeat center center;
    background-size: cover;
    display: flex;
    justify-content: center;

    .wox-launcher {
      height: ${props => props.height}px;
      margin-top: 32px;
      width: 800px;
      border-radius: 10px;
      background-color: ${props => props.theme.AppBackgroundColor};
      padding-top: ${props => props.theme.AppPaddingTop}px;
      padding-right: ${props => props.theme.AppPaddingRight}px;
      padding-bottom: ${props => props.theme.AppPaddingBottom}px;
      padding-left: ${props => props.theme.AppPaddingLeft}px;
      overflow: hidden;
    }

    .wox-launcher:before {
      content: "";
      width: 806px;
      height: ${props => props.height + 10}px;
      background: inherit;
      position: absolute;
      left: max(calc(50% - 404px), 0px);
      right: 0;
      top: 30px;
      bottom: 0;
      box-shadow: inset 0 0 0 200px rgba(255, 255, 255, 0.3);
      filter: blur(10px);
    }
  }
`
