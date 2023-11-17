import { ScrollbarProps, Scrollbars } from "react-custom-scrollbars"
import React, { useImperativeHandle, useRef } from "react"


export type WoxScrollbarRefHandler = {
  scrollTop(top: number): void
}

export type WoxScrollbarProps = {
  scrollbarProps: ScrollbarProps
  children?: React.ReactNode
  className?: string
}

export default React.forwardRef((_props: WoxScrollbarProps, ref: React.Ref<WoxScrollbarRefHandler>) => {
  const scrollbarRef = useRef<Scrollbars>(null)
  const { children, className, scrollbarProps } = _props

  useImperativeHandle(ref, () => ({
    scrollTop(top: number) {
      scrollbarRef.current?.scrollTop(top)
    }
  }))

  return <Scrollbars ref={scrollbarRef} className={`${className} wox-scroll-bar`} {...scrollbarProps} autoHeight={true} autoHeightMin={0}>{children}</Scrollbars>
})