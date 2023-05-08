# State applies
1. first, determine the root path and then check to see if a state file exists, or error out
1. next, refresh properties/pillars for each target
1. next, render the file using golang template rendering, *rendering magic* and then return the rendered bytes/string
1. next, check to ensure a unique state namespace for safe requisite inclusions
1. next, build dependency graph
1. check for cycles
1. test run all recipies to determine changes
1. run recipies in order in parallel (workgroups/goroutines?)
1. collect output and return to server


1. output returns job id at the bottom
1. errors are colorized but also called out at bottom for each target and also bottom of summary



