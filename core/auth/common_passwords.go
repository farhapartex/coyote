package auth

import "strings"

const commonPasswordList = `123456 password 123456789 12345678 12345 1234567 1234567890 qwerty abc123 111111
123123 000000 iloveyou 1234 1q2w3e4r qwertyuiop 123321 password1 password123 654321
666666 987654321 123 555555 3rjs1la7qe google 1q2w3e4r5t 123qwe zxcvbnm 1q2w3e
aa123456 dragon monkey letmein login princess qwe123 solo passw0rd starwars master
hello freedom whatever qazwsx trustno1 jordan harley ranger buster thomas
tigger robert soccer batman test pass killer hockey george charlie
andrew michelle love sunshine jessica asshole 6969 pepper daniel access
123456a joshua maggie 696969 shadow william fuckme 121212 heather hunter
fuck bailey amanda summer sophie ashley nicole chelsea biteme matthew
pussy 1qaz2wsx football baseball welcome admin adminadmin administrator root toor
guest user default changeme secret temp temp123 abc abcd abcd1234
qwerty123 asdfgh asdfghjkl zaq12wsx 1qazxsw2 qazxsw michael jennifer computer internet
samsung facebook myspace yahoo hotmail gmail linkedin twitter amazon netflix
purple orange yellow silver golden banana chocolate cookie ginger cheese
matrix phoenix jackson mustang corvette ferrari porsche yamaha honda harleydavidson
winter spring autumn january february october november december monday friday
apple mango cherry pumpkin peanut sunflower rainbow butterfly dolphin tiger
lovely angel angels flower flowers happy smile lucky rock rocket
superman spiderman batman1 pokemon minecraft fortnite roblox skywalker jedi vader
liverpool arsenal chelsea1 barcelona madrid juventus united city rangers celtic
london paris berlin madrid1 tokyo sydney canada america texas florida
iloveu ilovegod jesus christ heaven angel1 blessed faith hope grace
qwertz azerty poiuyt lkjhgf mnbvcxy 987654 456789 147258 258369 159753
abcdef abcde 11111111 22222222 88888888 77777777 99999999 12341234 11223344 10101010
passwort contrasena motdepasse senha wachtwoord parola haslo losen adgangskode salasana
letmein1 letmein123 open sesame opensesame trustme secret1 mysecret private nothing
whatever1 nopass nopassword none null undefined empty blank changeit change
p@ssw0rd p@ssword pa55word passw0rd1 s3cr3t adm1n r00t t3st qwer1234 asd123
1234abcd a1b2c3 a1b2c3d4 abc12345 test123 test1234 demo demo123 sample example`

func CommonPasswords() []string {
	return strings.Fields(commonPasswordList)
}
