# How we work together

## Weekly Sync

We run a Weekly Sync at [6pm UTC Tuesdays](https://www.wolframalpha.com/input/?i=Next+Tuesday+6pm+UTC) on [Zoom Room](https://protocol.zoom.us/j/299213319), notes are taken at [hackmd.io test-ground-weekly/edit](https://hackmd.io/@daviddias/test-ground-weekly/edit?both) and stored at [meeting-notes](https://github.com/ipfs/testground/tree/master/_meeting-notes). This weekly is listed on the [IPFS Community Calendar](https://github.com/ipfs/community#community-calendar). Recordings can be found [here](https://drive.google.com/open?id=1VL57t9ZOtk5Yw-cQoG7TtKaf3agDsrLc)(currently only available to the team).

For each Weeky Sync, each contributor is responsible for:

- Sharing an update on what was achieved, what got blocked and also what didn't get delivered and that was scheduled from the previous week assignments. It's fine to say "I didn't do X", there can be a multitude of reasons, from got blocked, didn't get time, etc. Even if it is a "I wasn't able to", that's fine as well as we will be able to identify it and pair you with someone that can help you learn how to do it.
- Coming to the meeting with a set of tasks that you will be focusing int he following week (3 is a good number as a minimum). At the end of the meeting (last 15 minutes) we review those together and update if needed.
- As we move through the Questions, each person that gets assigned an Action Items (AIs), has to c&p to their own assignments list.

This guidelines make us great Weekly Sync citizens :)

## Work Tracker

We track our work Kanban style in a [Zenhub board](https://app.zenhub.com/workspaces/test-ground-5db6a5bf7ca61c00014e2961/board?repos=197244214) (plus, if you want to give your browser super powers, get the [Zenhub extension](https://www.zenhub.com/extension)). Notes on using the Kanban:
- The multiple stages are:
  - **Inbox** - New issues or PRs that haven't been evaluated yet
  - **Icebox** - Low priority, un-prioritized Issues that are not immediate priorities.
  - **Blocked** - Issues that are blocked or discussion threads that are not currently active
  - **Backlog** - Upcoming Issues that are immediate priorities. Issues here should be prioritized top-to-bottom in the pipeline.
  - **In Progress** - Issues that someone is already tackling. Contributors should focus on a few things rather than many at once.
  - **Review/QA** - Issues open to the team for review and testing. Code is ready to be deployed pending feedback.
  - **OKR** - This column is just a location for the OKR cards to live until all the work under them is complete.
  - **Closed/Done** - Issues are automatically moved here when the issue is closed or the PR merged. Means that the work of the issue has been complete.
- We label issues using the following guidelines:
  - `difficulty:{easy, moderate, hard}` - This is an instinctive measure give by the project lead, project maintainer and/or architect.. It is a subjective best guess, however the current golden rule is that an issue with difficulty:easy should not require more than a morning (3~4 hours) to do and it should not require having to mess with multiple modules to complete. Issues with difficulty moderate or hard might require some discussion around the problem or even request that another team (i.e go-ipfs) makes some changes. The length of moderate or hard issue might be a day to ad-aeternum.
  - `priority (P0, P1, P2, P3, P4)` - P0 is the most important while P4 is the least.
  - `good first issue` - Issues perfect for new contributors. They will have the information necessary or the pointers for a new contributor to figure out what is required. These issues are never blocked on some other issue be done first.
  - `help wanted` - A label to flag that the owner of the issue is looking for support to get this issue done.
-   `blocked` - Work can't progress until a dependency of the issue is resolved.
- Responsibilities:
  - Project Maintainer and/or Project Architect - Review issues on Inbox, break them down if necessary, move them into Ready when it is the right time. Also, label issues with priority and difficulty.
  - Contributors move issues between the Ready, In Progress and Review/QA Colums. Use help wanted and blocked labels in case they want to flag that work.
  
## Contributing Guildeines 

Read [CONTRIBUTING.md](https://github.com/ipfs/testground/blob/master/CONTRIBUTING.md)
