#!/usr/bin/env python3
"""生成一个用于验证模拟赛评测的目录：3 道题、8 位选手、几十份提交。"""
import os
import random
import shutil
import sys

ROOT = sys.argv[1] if len(sys.argv) > 1 else "/tmp/otk-contest-demo"
if os.path.isdir(ROOT):
    shutil.rmtree(ROOT)

random.seed(20260214)

# ---- 题目定义：名字 -> (生成测试数据, 正解, 各种"错误解") ----------------
# aplusb: 求和
# maxsub: 最大子段和
# sortarr: 排序后输出


def gen_aplusb(path):
    os.makedirs(path, exist_ok=True)
    for i in range(1, 11):
        a, b = random.randint(-10**9, 10**9), random.randint(-10**9, 10**9)
        with open(f"{path}/{i}.in", "w") as f:
            f.write(f"{a} {b}\n")
        with open(f"{path}/{i}.ans", "w") as f:
            f.write(f"{a + b}\n")


def gen_maxsub(path):
    os.makedirs(path, exist_ok=True)
    for i in range(1, 9):
        n = 10 if i <= 4 else 200
        arr = [random.randint(-100, 100) for _ in range(n)]
        with open(f"{path}/{i}.in", "w") as f:
            f.write(f"{n}\n{' '.join(map(str, arr))}\n")
        best = cur = arr[0]
        for x in arr[1:]:
            cur = max(x, cur + x)
            best = max(best, cur)
        with open(f"{path}/{i}.ans", "w") as f:
            f.write(f"{best}\n")


def gen_sortarr(path):
    os.makedirs(path, exist_ok=True)
    for i in range(1, 7):
        n = random.randint(5, 50)
        arr = [random.randint(-1000, 1000) for _ in range(n)]
        with open(f"{path}/{i}.in", "w") as f:
            f.write(f"{n}\n{' '.join(map(str, arr))}\n")
        with open(f"{path}/{i}.ans", "w") as f:
            f.write(" ".join(map(str, sorted(arr))) + "\n")


PROBLEMS = {
    "aplusb": gen_aplusb,
    "maxsub": gen_maxsub,
    "sortarr": gen_sortarr,
}

# ---- 各种实现：正确 / 部分正确 / 编译错误 / 超时 / 运行错误 --------------
S = {}

S["aplusb_ac"] = """#include <bits/stdc++.h>
int main(){long long a,b;std::cin>>a>>b;std::cout<<a+b<<std::endl;}
"""

S["aplusb_int"] = """#include <bits/stdc++.h>
int main(){int a,b;std::cin>>a>>b;std::cout<<a+b<<std::endl;}
"""

S["aplusb_ce"] = """#include <bits/stdc++.h>
int main(){ this is not c++ }
"""

S["maxsub_ac"] = """#include <bits/stdc++.h>
int main(){int n;std::cin>>n;std::vector<long long>a(n);for(auto&x:a)std::cin>>x;
long long best=a[0],cur=a[0];for(int i=1;i<n;i++){cur=std::max<long long>(a[i],cur+a[i]);best=std::max(best,cur);}
std::cout<<best<<std::endl;}
"""

S["maxsub_slow"] = """#include <bits/stdc++.h>
int main(){int n;std::cin>>n;std::vector<long long>a(n);for(auto&x:a)std::cin>>x;
long long best=LLONG_MIN;for(int i=0;i<n;i++){long long s=0;for(int j=i;j<n;j++){s+=a[j];best=std::max(best,s);}}
std::cout<<best<<std::endl;}
"""

S["maxsub_re"] = """#include <bits/stdc++.h>
int main(){int n;std::cin>>n;std::vector<int>a(n);for(auto&x:a)std::cin>>x;
int*p=nullptr;std::cout<<a[0]<<p[0]<<std::endl;}
"""

S["sortarr_ac"] = """#include <bits/stdc++.h>
int main(){int n;std::cin>>n;std::vector<int>a(n);for(auto&x:a)std::cin>>x;
std::sort(a.begin(),a.end());for(int i=0;i<n;i++)std::cout<<a[i]<<" \\n"[i==n-1];}
"""

S["sortarr_wa"] = """#include <bits/stdc++.h>
int main(){int n;std::cin>>n;std::vector<int>a(n);for(auto&x:a)std::cin>>x;
std::sort(a.begin(),a.end());for(int i=0;i<n;i++)std::cout<<a[i]<<" ";std::cout<<std::endl;}
"""

# 8 位选手，每人 3 道题；不同选手拿不同实现，模拟真实分布。
PLAYERS = {
    "张三": ("aplusb_ac", "maxsub_ac", "sortarr_ac"),
    "李四": ("aplusb_ac", "maxsub_slow", "sortarr_ac"),
    "王五": ("aplusb_int", "maxsub_ac", "sortarr_wa"),
    "赵六": ("aplusb_ac", "maxsub_re", "sortarr_ac"),
    "陈七": ("aplusb_ce", "maxsub_ac", "sortarr_ac"),
    "周八": ("aplusb_ac", "maxsub_slow", "sortarr_wa"),
    "吴九": ("aplusb_ac", "maxsub_ac", "sortarr_wa"),
    "郑十": ("aplusb_int", "maxsub_re", "sortarr_ac"),
}

data_dir = os.path.join(ROOT, "data")
src_dir = os.path.join(ROOT, "source")
for name, gen in PROBLEMS.items():
    gen(os.path.join(data_dir, name))

count = 0
for player, impls in PLAYERS.items():
    pdir = os.path.join(src_dir, player)
    os.makedirs(pdir, exist_ok=True)
    for prob, impl in zip(PROBLEMS, impls):
        with open(os.path.join(pdir, prob + ".cpp"), "w") as f:
            f.write(S[impl])
        count += 1

print(f"已生成 {ROOT}")
print(f"  {len(PROBLEMS)} 道题，{len(PLAYERS)} 位选手，{count} 份提交")
for name in PROBLEMS:
    n = len([f for f in os.listdir(os.path.join(data_dir, name)) if f.endswith(".in")])
    print(f"  {name}: {n} 个测试点")
