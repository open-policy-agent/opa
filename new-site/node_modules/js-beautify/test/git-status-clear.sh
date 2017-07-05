echo "Post-build git status check..."
echo "Ensuring no changes visible to git have been made to '$*' ..."

git status $* | grep "nothing to commit.*working directory clean" || {
    # we should find nothing to commit. If we don't, build has failed. 
    echo "ERROR: Post-build git status check - FAILED."
    echo "Git status reported changes to non-git-ignore'd files."
    echo "TO REPRO: Run 'git status $*'.  The use git gui or git diff to see what was changed during the build."
    echo "TO FIX: Amend your commit and rebuild. Repeat until git status reports no changes both before and after the build."
    echo "OUTPUT FOR 'git status $*':" 
    git status $*
    exit 1
}
echo "Post-build git status check - Succeeded."
exit 0
