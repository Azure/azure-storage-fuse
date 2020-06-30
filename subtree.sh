

echo "Performing $1 operation"
echo "--------------------------------"
echo

if [ $1 = "pull" ]
then
  git subtree pull --prefix cpplite  https://github.com/Azure/azure-storage-cpplite master --squash
elif [ $1 = "push" ]
then
  git subtree push --prefix cpplite  https://github.com/Azure/azure-storage-cpplite master --squash
elif [ $1 = "add" ]
then
  git subtree add --prefix cpplite  https://github.com/Azure/azure-storage-cpplite master --squash
elif [ $1 = "remove" ]
then
  git rm -f cpplite
elif [ $1 == "list" ]
then
  echo "List of sbutree directories : "
  git log | grep git-subtree-dir | awk '{ print $2 }'
elif [ $1 == "diff" ]
then
  echo "Diff between local subtree and remote master : "
  git diff azure-storage-cpplite/master master:cpplite
elif [ $1 == "remote-tree" ]
then  
  echo "Adding remote git for subtree : "
  git remote add -f azure-storage-cpplite https://github.com/Azure/azure-storage-cpplite
fi
echo

