Build a desktop Markdown editor and viewer written in Go using the Fyne framework. 
The app should open, edit, preview, and save `.md` files.

Additional requirements:

1. The editor should accept a command line paremeter:
   a file name to open for editing at program's start.

2. Add a button for exporting the MD file to HTML format in the same directory as the MD file.

3. "Open" button opens a dialogue window for choosing file in the HOME directory.
   It should show the CURRENT WORKING directory instead.

4. Implement the "Preview-only mode": when the program is started with --preview parameter and the file name, it should hide the editor panel and show only the rendered MD file.

5. Implement the support for i18n: all the window and widget captions and messages should be prepared  for localization as it's suggested in Fyne.
 3 languages should be supported: English (default), Russian and Japanese.

6. Add a "Web View" button (don't forget to do the corresponding changes for i18n): it should transform the current MD into a temporary HTML file and then open it in the system's default browser for viewing as a web page. The temporary file should be erased on exiting the editor.

7. HTML generated for export and web view should render all document text with a sans-serif font.

8. Implement a synchronous scrolling of the Editor (left) and Preview (right) panels:
-  when you scroll the text in the left panel, it should be automatically scrolled in the right panel,
   so that corresponding lines are shown in roughly the same position;
- and vice versa.
