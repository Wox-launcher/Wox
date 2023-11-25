import styled from "styled-components"
import WoxQueryBox from "../query/WoxQueryBox.tsx"
import { Theme } from "../../entity/Theme.typings"
import { WoxThemeHelper } from "../../utils/WoxThemeHelper.ts"
import WoxQueryResult from "../query/WoxQueryResult.tsx"
import { WOX_QUERY_RESULT_ITEM_HEIGHT } from "../../utils/WoxConst.ts"

export default () => {
  const getLaucherHeight = () => {
    const theme = WoxThemeHelper.getInstance().getTheme()
    const baseItemHeight = WOX_QUERY_RESULT_ITEM_HEIGHT + theme.ResultItemPaddingTop + theme.ResultItemPaddingBottom
    return 50 + baseItemHeight * 3 + theme.ResultContainerPaddingTop + theme.ResultContainerPaddingBottom + theme.AppPaddingTop + theme.AppPaddingBottom
  }

  return (
    <Style theme={WoxThemeHelper.getInstance().getTheme()} launcherHeight={getLaucherHeight()}>
      <div className={"wox-preview-launcher"}>
        <div className={"wox-launcher"}>
          <WoxQueryBox defaultValue={"Hello World!"} isPreview={true} />
          <WoxQueryResult isPreview={true} />
        </div>
      </div>
    </Style>
  )
}

const Style = styled.div<{ theme: Theme; launcherHeight: number }>`
  .wox-preview-launcher {
    position: relative;
    width: 100%;
    height: 300px;
    background: url("/wox-preview-bg.jpg") no-repeat center center;
    background-size: cover;
    display: flex;
    justify-content: center;

    .wox-launcher {
      height: ${props => props.launcherHeight}px;
      margin-top: 32px;
      max-width: 800px;
      min-width: 600px;
      background-color: ${props => props.theme.AppBackgroundColor};
      padding-top: ${props => props.theme.AppPaddingTop}px;
      padding-right: ${props => props.theme.AppPaddingRight}px;
      padding-bottom: ${props => props.theme.AppPaddingBottom}px;
      padding-left: ${props => props.theme.AppPaddingLeft}px;
      overflow: hidden;
    }
  }
`
