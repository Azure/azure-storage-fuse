

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
fi
echo

