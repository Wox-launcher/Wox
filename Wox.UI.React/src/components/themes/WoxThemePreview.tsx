import styled from "styled-components"
import WoxQueryBox from "../query/WoxQueryBox.tsx"
import { Theme } from "../../entity/Theme.typings"
import { WoxThemeHelper } from "../../utils/WoxThemeHelper.ts"
import WoxQueryResult from "../query/WoxQueryResult.tsx"
import { WOX_QUERY_RESULT_ITEM_HEIGHT } from "../../utils/WoxConst.ts"
import { WOXMESSAGE } from "../../entity/WoxMessage.typings"

export default (props: { theme: Theme }) => {
  const getInitResultList = () => {
    return [
      {
        QueryId: "641a84d9-e832-4976-9af8-71552e0f7799",
        Id: "cb792d11-dfd4-4899-87df-42d10c18979c",
        Title: "VoiceOver",
        SubTitle: "/System/Library/CoreServices/VoiceOver.app",
        Icon: { ImageType: "url", ImageData: "/voice_over.png" } as WOXMESSAGE.WoxImage,
        Score: 8,
        Preview: {},
        ContextData: "",
        Actions: [
          {
            Id: "e5ec9374-facf-4733-9439-a3b9e067c4c1",
            Name: "打开",
            Icon: {
              ImageType: "svg",
              ImageData:
                '<svg xmlns="http://www.w3.org/2000/svg" x="0px" y="0px" width="64" height="64" viewBox="0 0 32 32"><polygon fill="#0f518c" points="30,30 2,30 2,2 17,2 17,6 6,6 6,26 26,26 26,15 30,15"></polygon><polygon fill="#ed0049" points="19,2 19,6 23.172,6 14.586,14.586 17.414,17.414 26,8.828 26,13 30,13 30,2"></polygon></svg>'
            },
            IsDefault: true,
            PreventHideAfterAction: false
          },
          {
            Id: "115697fe-a395-4254-aaed-a8b570848df5",
            Name: "打开所在文件夹",
            Icon: {
              ImageType: "svg",
              ImageData:
                '<svg xmlns="http://www.w3.org/2000/svg" x="0px" y="0px" width="48" height="48" viewBox="0 0 48 48"><path fill="#FFA000" d="M40,12H22l-4-4H8c-2.2,0-4,1.8-4,4v8h40v-4C44,13.8,42.2,12,40,12z"></path><path fill="#FFCA28" d="M40,12H8c-2.2,0-4,1.8-4,4v20c0,2.2,1.8,4,4,4h32c2.2,0,4-1.8,4-4V16C44,13.8,42.2,12,40,12z"></path></svg>'
            },
            IsDefault: false,
            PreventHideAfterAction: false
          },
          {
            Id: "ec482e87-099b-49fa-a9b7-8e1829abe1ce",
            Name: "复制路径",
            Icon: {
              ImageType: "base64",
              ImageData:
                "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAACXBIWXMAAAsTAAALEwEAmpwYAAAA+0lEQVR4nO3VLQ7CQBAF4L3Eih6HOQScAIfmDMhq0NVgcegaCAq7GoMpBJKyBEc2/GyZaafTzkue36/7sjVGo9FoICs8toOsmLF9Sezhh8szLwIL2B5LP1oxIrAAV9x5ERQAR41I86vHdrK+VAI4SgT28PPdLRrhXgBkCCzgcCr9IhLhAgAJAgt4HiIW4d4AQgQLoAoCfpQNQIUwnAAKhOEGYBGmDYAQAW0GxNSONx+rgLSBGwDpEwLpAPtl87UAkun+73YSANInBNIBtun/AHYynQOA9AmBdICV/oxa6QCQPiFQQN+e0UQ6ICWuqTMKyPUGej4hjUZjROQBwgDUDcPYwFwAAAAASUVORK5CYII="
            },
            IsDefault: false,
            PreventHideAfterAction: false
          }
        ],
        RefreshInterval: 0
      },
      {
        QueryId: "641a84d9-e832-4976-9af8-71552e0f7799",
        Id: "cb792d11-dfd4-4899-87df-42d10c18980c",
        Title: "Freeform",
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
        Title: "Screenshot",
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
    return 50 + baseItemHeight * 6 + theme.ResultContainerPaddingTop + theme.ResultContainerPaddingBottom + theme.AppPaddingTop + theme.AppPaddingBottom
  }

  return (
    <Style theme={props.theme} height={getLauncherHeight()}>
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
    height: 440px;
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
