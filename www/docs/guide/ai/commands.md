# AI Commands

Wox allows you to integrate AI capabilities directly into your workflow.

## Auto Git Commit Message

This feature allows you to automatically generate a commit message using Wox AI Command from terminal.

### Setup

1. **Add AI command**

   - Query `aicommand` in Wox and select `Open AI Commands settings`
   - In the Settings tab, click on the `Add` button
   - Add the following information:

     - **Name**: `git commit msg`
     - **Query**: `commit`
     - **Model**: `<your choice>`
     - **Prompt**:

       ```
       Below I will give you a git diff output, please help me write a commit msg targeting these changes. Requirements are as follows:
        - The first line should be a description no more than 50 words, followed by a blank line, and then 2-3 detailed descriptions.
        - Do not respond with anything except the commit msg.

       Here is my input:
       %s
       ```

     - **Vision**: No

   ![AI git msg setting](../../data/images/ai_auto_git_msg_setting.png)

2. **Config bash scripts**
   To use this feature, you can add the following script to your `.bashrc` or `.zshrc` file:

   ```bash
   commit() {
       open "wox://query?q=ai commit $(cat)"
   }
   ```

### Usage

After setting up, you can use this feature in your git project by executing the following command:

```bash
git diff | commit
```

This command will automatically call Wox and generate a commit message for you.

![AI git msg](../../data/images/ai_auto_git_msg.png)
