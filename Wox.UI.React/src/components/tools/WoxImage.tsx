import { WoxImageTypeEnum } from "../../enums/WoxImageTypeEnum.ts"
import { WOXMESSAGE } from "../../entity/WoxMessage.typings"
import styled from "styled-components"

export function parseWoxImage(data: string): WOXMESSAGE.WoxImage {
  const img = {} as WOXMESSAGE.WoxImage
  if (data.startsWith("base64:")) {
    img.ImageType = WoxImageTypeEnum.WoxImageTypeBase64.code
    img.ImageData = data.slice(7)
    return img
  }
  if (data.startsWith("svg:")) {
    img.ImageType = WoxImageTypeEnum.WoxImageTypeSvg.code
    img.ImageData = data.slice(4)
    return img
  }
  if (data.startsWith("url:")) {
    img.ImageType = WoxImageTypeEnum.WoxImageTypeUrl.code
    img.ImageData = data.slice(4)
    return img
  }

  return img
}

export default (props: { img: WOXMESSAGE.WoxImage; height: number; width: number }) => {
  return (
    <Style width={props.width} height={props.height}>
      {props.img.ImageType === WoxImageTypeEnum.WoxImageTypeSvg.code && <div className={"wox-image"} dangerouslySetInnerHTML={{ __html: props.img.ImageData }}></div>}
      {props.img.ImageType === WoxImageTypeEnum.WoxImageTypeUrl.code && <img src={props.img.ImageData} className={"wox-image"} alt={"wox-image"} />}
      {props.img.ImageType === WoxImageTypeEnum.WoxImageTypeBase64.code && <img src={props.img.ImageData} className={"wox-image"} alt={"wox-image"} />}
    </Style>
  )
}

const Style = styled.div<{ width: number; height: number }>`
  display: flex;
  height: ${props => props.height}px;
  width: ${props => props.width}px;
  justify-content: center;
  align-items: center;

  .wox-image {
    line-height: ${props => props.height}px;
    max-height: ${props => props.height}px;
    max-width: ${props => props.width}px;
    text-align: center;
    vertical-align: middle;

    svg {
      max-width: ${props => props.height}px !important;
      max-height: ${props => props.height}px !important;
    }
  }
`
