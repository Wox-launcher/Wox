# Wox blog writing style

Read this before drafting or substantially rewriting an article. Aim for the voice of a knowledgeable colleague explaining a useful workflow: concrete, thoughtful, and easy to follow. Give the reader enough context to understand the steps, without making a short tip feel like a manual.

## What the Mole examples demonstrate

These observations come from five Chinese articles read on 2026-10-20. They are editorial observations about this sample, not a claim that every Mole article uses one house style. Their technical and commercial claims are not facts about Wox.

| Example | Observed technique | How to adapt it |
| --- | --- | --- |
| [Temporarily keeping Windows awake](https://mole.fit/zh/windows/blog/keep-windows-awake) | Opens with an unfinished task and an inconvenient screen timeout. Explains the distinction the reader needs before introducing the product controls. | Start with an ordinary interruption or repeated action. Explain a concept only when it changes what the reader should do next. |
| [Disk activity at 100%](https://mole.fit/zh/windows/blog/windows-disk-100-percent) | Moves from an observed symptom to interpreting readings, locating the responsible activity, and checking a change. Uses a comparison table to answer distinct questions. | For troubleshooting, organize around evidence and observable results. Use tables for real comparisons; avoid lists of speculative fixes. |
| [Windows interface design](https://mole.fit/zh/windows/blog/mole-windows-design) | Makes design claims concrete through list contents, button labels, waiting states, and window behavior. Illustrations are tied to particular points. | Describe what the user sees and why a design choice helps in that moment. Support an adjective such as “clear” with an actual interaction. |
| [Developing the Windows app on a Mac](https://mole.fit/zh/windows/blog/building-mole-for-windows-on-a-mac) | Builds a personal account out of a real setup, constraints, failed attempts, and subsequent adjustments. Headings name the actual work. | In development stories, use supplied experience and verified artifacts. Never manufacture a first-person anecdote, test count, quotation, or mishap to sound human. |
| [CLI and desktop app comparison](https://mole.fit/zh/windows/blog/mole-cli-vs-windows-app) | Clarifies the confusing distinction early, compares concrete differences, and ends with choices for different users. | In comparisons, choose dimensions that affect the reader's decision. Explain when each option fits instead of assuming Wox must always win. |

The practical articles generally sound calm and explanatory; the development story is more personal and conversational. Borrow these techniques, not the author's identity or signature expressions. Some sampled tutorials contain long sentences and repeated caveats. Wox should keep the useful explanation while shortening those passages. The recurring promotional footer is site furniture, not a required article ending.

## Build substance from the reader's task

**Open with something recognizable.** Name the activity, the small inconvenience, and the useful outcome. A reader should recognize when they would use the feature before meeting a series of settings. Do not invent dramatic pain, a fictional customer, or a personal success story.

**Give the workflow a concrete example.** Carry one example through setup and use when it helps: a named workspace, a particular query, or selected text with a clear transformation. Explain what example values mean and which ones the reader should replace. Several unrelated use cases often add less than one developed example.

**Explain consequential choices.** Before or after a setting, explain why it is needed when the reason is not obvious. For example, a shortcut that merely runs a query and an AI command whose default action pastes a result have different responsibilities. Readers need that distinction to understand the outcome.

**Show the result.** After the command or final step, describe what appears, changes, or stays available. Put a screenshot beside the point it explains; give it a caption when a detail deserves attention. A screenshot should not have to carry an explanation the text omits.

**Place limits at the point of use.** Mention platform support, prerequisites, changed data, or a fallback where it affects the decision. Keep the weight proportional to the feature. A simple app-grid tip does not need a troubleshooting encyclopedia or unrelated safety notices.

**End when the task is complete.** A useful next step, an optional shortcut, or a relevant limitation can finish the piece. Do not append another summary of everything already said. Link to a related guide when it answers the next likely question.

These are reasoning checks, not mandatory sections. A small tip can fit in several connected paragraphs with one command. A multi-step workflow may need headings and screenshots. A development story can follow constraint, choice, attempt, and result when there is evidence for that sequence. Do not impose a word quota, fixed number of headings, FAQ, or universal conclusion.

## Titles, descriptions, and headings do different jobs

- The title names a recognizable task or result. Use a question when the article actually answers one; do not mechanically put “如何” or “How to” into every title.
- The card description adds the situation, scope, or useful outcome in a compact sentence. Keep the card scannable; develop the explanation in the article body.
- Headings tell readers what the next passage helps them understand or do. Prefer a specific action or decision over repeated labels such as “功能介绍 / 使用方法 / 注意事项 / 总结”.
- Use `Wox`, including in `Wox 博客` and `Wox Blog`. Do not turn the product name into `WOX` in prose or page titles.
- Do not attach the same introductory phrase to every post. Preserve old slugs when changing a title.

## Natural prose without artificial personality

Use concrete nouns and verbs: which window, which key, which setting, what happens next. Vary sentence length according to the explanation. Split a long Chinese sentence when the action, cause, or qualification changes; do not join an entire paragraph with commas. Use numbered steps for a sequence the reader will perform, a table for comparisons or field values, and prose for reasons and transitions.

Remove stock openings such as “在当今快节奏的数字时代” and claims such as “彻底改变工作方式”, “无缝赋能”, “效率神器”, or “释放潜能” when they do not convey a verifiable outcome. Do not replace them with another set of stock phrases. Apply the same judgment to English slogans such as “supercharge your workflow” and “game-changing”.

Avoid repeated rhetorical questions, decorative three-part lists, a bold takeaway in every paragraph, and a formulaic benefit sentence after every step. Ordinary useful words are not banned: the test is whether the sentence tells this reader something specific.

Warmth comes from recognizing a familiar task and explaining it patiently. Do not add “小伙伴”, “大伙”, “哈哈”, slang, fake complaints, or exclamation marks merely to imitate an individual author. First-person writing is appropriate when the user provides the experience or asks for an evidenced author account; otherwise use a neutral explanation.

Translate the intent naturally. Keep the command, setting, prerequisite, and outcome equivalent, but allow sentence boundaries and idioms to differ. Do not turn concise English into inflated marketing prose or force Chinese clause order into English.

## Original Wox examples

The examples below are editorial demonstrations based on existing Wox article topics, not replacements to apply automatically. Recheck feature behavior when publishing.

### An opening with enough context

Too thin:

> Wox 支持保存和恢复工作区布局。打开设置，新建工作区，再运行命令。

More useful:

> 开始写代码前，你可能会先把编辑器放到主屏，再把终端和浏览器挪到旁边。窗口一多，每次重新摆放都要花点时间。可以把这套安排保存成 Wox 工作区，下次从启动器里选中它，让这些窗口回到保存的位置。

The additional detail explains a recognizable use and the result. It does not need a claim about minutes saved or a fabricated personal routine.

### Explain a command's purpose

Too thin:

> 输入 `app launchpad`，可以提高应用启动效率。

More useful:

> 记得应用图标，却一时想不起名字时，可以输入 `app launchpad`。Wox 会把已安装的应用排成图标网格，方便你扫一眼再打开。已经知道名字时，直接输入名称搜索通常更省事。

The passage supplies a reason to choose the feature and a practical alternative, without claiming measured speed.

### Explain what a setting changes

Too thin:

> 开启静默执行，即可完成一键翻译。

More useful:

> 想让翻译结果替换原来的选中文本，先把 AI 命令的默认动作设为“运行并粘贴”，再把它绑定到快捷键查询。“静默执行”负责直接运行这个默认动作，替换文本则由“运行并粘贴”完成。Wox 会等最终结果返回后再粘贴；如果想先检查译文，就把默认动作设为“运行”。

This explains a non-obvious choice and its consequence. It belongs in a configuration walkthrough, not in every short tip.

## Editorial pass before delivery

Read the draft without its screenshots and ask:

- Can the reader identify a situation where this is useful, follow the example, and recognize success?
- Does each paragraph add a fact, reason, example, action, or consequence? Cut repeated benefits and generic transitions.
- Where the prose feels thin, is a useful reason or observable result missing? Add that information instead of adjectives or padding.
- Are anecdotes, measurements, quotations, comparisons, and claims of testing actually supported? Remove or qualify anything that is not.
- Does the organization fit this article, or does it look like the same template applied to every feature?
- Are qualifications nearby and proportionate, and are Chinese and English equally natural?

Do not rewrite existing posts merely because this guide changed. Apply it to the article the user requested.
