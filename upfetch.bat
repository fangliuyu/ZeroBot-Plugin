git remote add upstream git@github.com:FloatTech/ZeroBot-Plugin.git
git remote -v
git fetch upstream master
git merge upstream/master
git push -u origin master

git pull --tags -r origin master

pause