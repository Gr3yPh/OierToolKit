import org.jline.reader.LineReader;
import org.jline.reader.LineReaderBuilder;
import org.jline.terminal.Terminal;
import org.jline.terminal.TerminalBuilder;

import java.io.*;
import java.nio.file.*;
import java.util.*;
import java.util.stream.Collectors;

public class OierToolKit {
    private static final String RESET = "\u001B[0m";
    private static final String RED = "\u001B[31m";
    private static final String GREEN = "\u001B[32m";
    private static final String YELLOW = "\u001B[33m";
    private static final String BLUE = "\u001B[34m";
    private static final String CYAN = "\u001B[36m";

    private static final Path BASE_DIR = Paths.get(System.getProperty("user.home"), "otk");
    private static Path currentDir = BASE_DIR;
    private static String currentProject = "";

    public static void main(String[] args) {
        try {
            Files.createDirectories(BASE_DIR);
        } catch (IOException e) {
            System.out.println(RED + "无法创建根目录 ~/otk/" + RESET);
            return;
        }
        
        String startUpMes="""
###############################################################################
#            ____       ________    ________  ______   _______                #
#        ____\\_  \\__   /        \\  /        \\|\\     \\  \\      \\               #
#       /     /     \\ |\\         \\/         /|\\     \\  |     /|              #
#      /     /\\      || \\            /\\____/ | \\|     |/     //               #
#     |     |  |     ||  \\______/\\   \\     | |  |     |_____//                #
#     |     |  |     | \\ |      | \\   \\____|/   |     |\\     \\                #
#     |     | /     /|  \\|______|  \\   \\       /     /|\\|     |               #
#     |\\     \\_____/ |           \\  \\___\\     /_____/ |/_____/|               #
#     | \\_____\\   | /             \\ |   |    |     | / |    | |               #
#      \\ |    |___|/               \\|___|    |_____|/  |____|/                #
#       \\|____|                                                 OierToolKit   #
#                                                       v0.7 by Gr3yPh4ntom   #
###############################################################################
        """;
        System.out.println(CYAN + startUpMes + RESET);
        System.out.println("输入 'h' 查看帮助。输入 'cmd [命令]' 执行外部系统命令。\n");

        try {
            Terminal terminal = TerminalBuilder.builder().system(true).build();
            LineReader reader = LineReaderBuilder.builder().terminal(terminal).build();

            while (true) {
                String promptProject = currentProject.isEmpty() ? "~" : currentProject;
                String prompt = BLUE + "[otk @ " + promptProject + "]$ " + RESET;

                String line;
                try { line = reader.readLine(prompt); } 
                catch (org.jline.reader.UserInterruptException e) { continue; } 
                catch (org.jline.reader.EndOfFileException e) { break; }

                line = line.trim();
                if (line.isEmpty()) continue;

                if (line.startsWith("cmd ")) {
                    executeSystemCommand(line.substring(4).trim());
                    continue;
                }

                String[] tokens = line.split("\\s+");
                String cmd = tokens[0].toLowerCase();

                switch (cmd) {
                    case "c":
                    case "create":
                        if (tokens.length < 2) System.out.println(YELLOW + "用法: c [PROJECT]" + RESET);
                        else createProject(tokens[1]);
                        break;
                    case "s":
                    case "switch":
                        if (tokens.length < 2) System.out.println(YELLOW + "用法: s [PROJECT]" + RESET);
                        else switchProject(tokens[1]);
                        break;
                    case "d":
                    case "delete":
                        if (tokens.length < 2) System.out.println(YELLOW + "用法: d [PROJECT]" + RESET);
                        else deleteProject(tokens[1]);
                        break;
                    case "list":
                        listInfo();
                        break;
                    case "set":
                        if (currentProject.isEmpty()) System.out.println(RED + "请先进入一个项目！" + RESET);
                        else handleSetCommand(tokens);
                        break;
                    case "ne":
                        if (currentProject.isEmpty()) System.out.println(RED + "请先进入一个项目！" + RESET);
                        else createSample(reader);
                        break;
                    case "r":
                    case "run":
                        if (currentProject.isEmpty()) System.out.println(RED + "请先进入一个项目！" + RESET);
                        else runTest();
                        break;
                    case "author":
                        printAuthor();
                        break;
                    case "h":
                    case "help":
                        printHelp();
                        break;
                    case "q":
                    case "exit":
                    case "quit":
                        System.out.println(GREEN + "Goodbye, Oier!" + RESET);
                        return;
                    default:
                        System.out.println("未知命令: " + cmd + "。输入 'h' 查看帮助。");
                }
            }
        } catch (Exception e) {
            System.out.println(RED + "终端初始化失败: " + e.getMessage() + RESET);
        }
    }

    private static void printAuthor() {
        System.out.println("OierToolKit v0.7 by 魇珩Gr3yPh4ntom\n" +
                "（严格来讲应该叫OierOperationSimplifier才对吧。。）\n" +
                "https://gr3yph4ntom.cn");
    }

    private static void printHelp() {
        System.out.println("可用命令列表:");
        System.out.println("  c, create [PROJECT]  - 新建一个题目项目");
        System.out.println("  s, switch [PROJECT]  - 切换题目项目 (s ~ 返回根目录)");
        System.out.println("  d, delete [PROJECT]  - 删除指定项目");
        System.out.println("  list                 - 列出项目、配置与样例");
        System.out.println("  set [key] [value]    - 设置当前项目的配置 (支持 time / memory)");
        System.out.println("  ne                   - 新建测试样例");
        System.out.println("  r, run               - 编译并评测当前项目 (高精度时间/内存/Diff)");
        System.out.println("  cmd [SYS_CMD]        - 直接执行 Linux 系统命令 (e.g., cmd ls -la)");
        System.out.println("  author               - 查看作者信息");
        System.out.println("  h, help              - 显示帮助");
        System.out.println("  q, exit              - 退出 otk");
    }

    private static void executeSystemCommand(String sysCmd) {
        if (sysCmd.isEmpty()) {
            System.out.println(YELLOW + "用法: cmd [系统命令]" + RESET);
            return;
        }
        try {
            ProcessBuilder pb = new ProcessBuilder("bash", "-c", sysCmd);
            pb.directory(currentDir.toFile()); 
            pb.inheritIO();
            Process p = pb.start();
            p.waitFor();
        } catch (Exception e) {
            System.out.println(RED + "系统命令执行失败: " + e.getMessage() + RESET);
        }
    }

    private static void createProject(String proj) {
        Path projDir = BASE_DIR.resolve(proj);
        try {
            if (Files.exists(projDir)) {
                System.out.println("项目 " + proj + " 已存在，正在为您切换。");
                switchProject(proj);
                return;
            }
            Files.createDirectories(projDir);
            Files.writeString(projDir.resolve(proj + ".cpp"), 
                "#include <iostream>\nusing namespace std;\n\nint main() {\n    // Write code here\n    return 0;\n}\n");
            Files.writeString(projDir.resolve(proj + ".ini"), "time_limit=1.00\nmemory_limit=125\n");
            System.out.println(GREEN + "成功创建项目: " + proj + RESET);
            switchProject(proj);
        } catch (IOException e) {
            System.out.println(RED + "创建失败: " + e.getMessage() + RESET);
        }
    }

    private static void switchProject(String proj) {
        if (proj.equals("~")) {
            currentDir = BASE_DIR;
            currentProject = "";
            System.out.println("已返回根目录");
            return;
        }
        Path projDir = BASE_DIR.resolve(proj);
        if (Files.exists(projDir) && Files.isDirectory(projDir)) {
            currentDir = projDir;
            currentProject = proj;
            System.out.println("已切换至: " + proj);
        } else {
            System.out.println(RED + "项目 " + proj + " 不存在！" + RESET);
        }
    }

    private static void deleteProject(String proj) {
        Path projDir = BASE_DIR.resolve(proj);
        if (!Files.exists(projDir)) {
            System.out.println(RED + "项目不存在！" + RESET);
            return;
        }
        try {
            Files.walk(projDir).sorted(Comparator.reverseOrder()).map(Path::toFile).forEach(File::delete);
            System.out.println(GREEN + "已删除项目: " + proj + RESET);
            if (currentProject.equals(proj)) {
                currentDir = BASE_DIR;
                currentProject = "";
            }
        } catch (IOException e) {
            System.out.println(RED + "删除失败: " + e.getMessage() + RESET);
        }
    }

    private static void listInfo() {
        if (currentProject.isEmpty()) {
            System.out.println("=== 现有项目列表 ===");
            try (var stream = Files.list(BASE_DIR)) {
                List<Path> projects = stream.filter(Files::isDirectory).collect(Collectors.toList());
                if (projects.isEmpty()) {
                    System.out.println("（暂无项目，使用 create [PROJECT] 新建一个吧）");
                } else {
                    for (Path p : projects) System.out.println("  * " + p.getFileName());
                }
            } catch (IOException e) {
                System.out.println(RED + "无法读取目录: " + e.getMessage() + RESET);
            }
        } else {
            System.out.println("=== 项目 [" + currentProject + "] 的详细状态 ===");
            Path iniPath = currentDir.resolve(currentProject + ".ini");
            if (Files.exists(iniPath)) {
                System.out.println("可用配置项:");
                try (FileInputStream fis = new FileInputStream(iniPath.toFile())) {
                    Properties props = new Properties();
                    props.load(fis);
                    System.out.println("  time_limit   : " + props.getProperty("time_limit", "1.00") + "s");
                    System.out.println("  memory_limit : " + props.getProperty("memory_limit", "125") + "MB");
                } catch (Exception e) {
                    System.out.println(YELLOW + "  (无法读取配置文件)" + RESET);
                }
            }
            File[] inFiles = currentDir.toFile().listFiles((dir, name) -> name.endsWith(".in"));
            System.out.println("已录入样例:");
            if (inFiles == null || inFiles.length == 0) {
                System.out.println("  （无可用样例，使用 'ne' 录入）");
            } else {
                Arrays.sort(inFiles, Comparator.comparing(File::getName));
                for (File f : inFiles) {
                    String id = f.getName().substring(0, f.getName().lastIndexOf('.'));
                    System.out.println("  样例 #" + id);
                }
            }
        }
    }

    private static void handleSetCommand(String[] tokens) {
        if (tokens.length < 3) {
            System.out.println(YELLOW + "用法: set time [秒数]  或  set memory [MB大小]" + RESET);
            return;
        }
        String key = tokens[1].toLowerCase();
        String value = tokens[2];
        Path iniPath = currentDir.resolve(currentProject + ".ini");

        try {
            Properties props = new Properties();
            if (Files.exists(iniPath)) {
                try (FileInputStream fis = new FileInputStream(iniPath.toFile())) { props.load(fis); }
            }
            if (key.equals("time")) {
                props.setProperty("time_limit", value);
                System.out.println(GREEN + "设置成功: 时间限制已调整为 " + value + "s" + RESET);
            } else if (key.equals("memory") || key.equals("mem")) {
                props.setProperty("memory_limit", value);
                System.out.println(GREEN + "设置成功: 内存限制已调整为 " + value + "MB" + RESET);
            } else {
                System.out.println(YELLOW + "未知配置项: " + key + " (支持: time / memory)" + RESET);
                return;
            }
            try (FileOutputStream fos = new FileOutputStream(iniPath.toFile())) {
                props.store(fos, "Updated by otk");
            }
        } catch (Exception e) {
            System.out.println(RED + "保存配置失败: " + e.getMessage() + RESET);
        }
    }

    private static void createSample(LineReader reader) {
        int id = 1;
        while (Files.exists(currentDir.resolve(id + ".in"))) id++;

        System.out.println("请输入样例输入 (单独一行输入 'eof' 结束):");
        String inputData = readUntilEOF(reader);

        System.out.println("请输入样例输出 (单独一行输入 'eof' 结束):");
        String outputData = readUntilEOF(reader);

        try {
            Files.writeString(currentDir.resolve(id + ".in"), inputData);
            Files.writeString(currentDir.resolve(id + ".out"), outputData);
            System.out.println(GREEN + "成功添加样例 #" + id + RESET);
        } catch (IOException e) {
            System.out.println(RED + "保存样例失败: " + e.getMessage() + RESET);
        }
    }

    private static String readUntilEOF(LineReader reader) {
        StringBuilder sb = new StringBuilder();
        while (true) {
            String line = reader.readLine("  > ");
            if (line.trim().equalsIgnoreCase("eof")) break;
            sb.append(line).append("\n");
        }
        return sb.toString();
    }

    private static void runTest() {
        String cppName = currentProject + ".cpp";
        Path cppPath = currentDir.resolve(cppName);
        Path exePath = currentDir.resolve(currentProject + ".out");
        Path timeTmpPath = currentDir.resolve(".time.tmp");

        if (!Files.exists(cppPath)) {
            System.out.println(RED + "错误: 未找到源码文件 " + cppName + RESET);
            return;
        }

        System.out.print("正在编译 " + cppName + "... ");
        try {
            ProcessBuilder pb = new ProcessBuilder("g++", "-O2", cppPath.toString(), "-o", exePath.toString());
            Process p = pb.start();
            if (p.waitFor() != 0) {
                System.out.println(RED + "[ CE ] 编译失败！" + RESET);
                BufferedReader br = new BufferedReader(new InputStreamReader(p.getErrorStream()));
                String line;
                while ((line = br.readLine()) != null) System.out.println("  " + RED + line + RESET);
                return;
            }
            System.out.println(GREEN + "编译成功" + RESET);
        } catch (Exception e) {
            System.out.println(RED + "编译异常: " + e.getMessage() + RESET);
            return;
        }

        double timeLimit = 1.00;
        double memLimit = 125.0; // MB
        try {
            Path iniPath = currentDir.resolve(currentProject + ".ini");
            if (Files.exists(iniPath)) {
                Properties props = new Properties();
                try (FileInputStream fis = new FileInputStream(iniPath.toFile())) { props.load(fis); }
                timeLimit = Double.parseDouble(props.getProperty("time_limit", "1.00"));
                memLimit = Double.parseDouble(props.getProperty("memory_limit", "125"));
            }
        } catch (Exception ignored) {}

        File[] inFiles = currentDir.toFile().listFiles((dir, name) -> name.endsWith(".in"));
        if (inFiles == null || inFiles.length == 0) {
            System.out.println(YELLOW + "提示: 未找到测试样例。请先使用 'ne' 创建。" + RESET);
            return;
        }

        Arrays.sort(inFiles, Comparator.comparing(File::getName));
        System.out.println(String.format("开始评测 (限制: %.2fs / %.0fMB):", timeLimit, memLimit));

        for (File inFile : inFiles) {
            String name = inFile.getName();
            String id = name.substring(0, name.lastIndexOf('.'));
            File outFile = new File(currentDir.toFile(), id + ".out");

            System.out.print("  样例 #" + id + " : ");
            if (!outFile.exists()) {
                System.out.println(YELLOW + "SKIP (缺少对应的 .out 文件)" + RESET);
                continue;
            }

            try {
                ProcessBuilder pb = new ProcessBuilder("/usr/bin/time", "-f", "%e %M", "-o", timeTmpPath.toString(), exePath.toString());
                pb.redirectInput(inFile);

                long startNano = System.nanoTime();
                Process p = pb.start();

                boolean exited = p.waitFor((long) (timeLimit * 1000), java.util.concurrent.TimeUnit.MILLISECONDS);
                long endNano = System.nanoTime();

                if (!exited) {
                    p.destroyForcibly();
                    System.out.println(RED + "TLE" + RESET + String.format(" (>%.0fms)", timeLimit * 1000));
                    continue;
                }

                if (p.exitValue() != 0) {
                    System.out.println(RED + "RE" + RESET + " (Exit Code: " + p.exitValue() + ")");
                    continue;
                }

                double runTimeSec = (endNano - startNano) / 1_000_000_000.0;
                double runMemMB = 0.0;
                if (Files.exists(timeTmpPath)) {
                    try {
                        String timeReport = Files.readString(timeTmpPath).trim();
                        String[] parts = timeReport.split("\\s+");
                        if (parts.length >= 2) {
                            runTimeSec = Double.parseDouble(parts[0]);
                            runMemMB = Double.parseDouble(parts[1]) / 1024.0; // KB -> MB
                        }
                    } catch (Exception ignored) {}
                }

                long runTimeMs = Math.round(runTimeSec * 1000);

                if (runMemMB > memLimit) {
                    System.out.println(RED + "MLE" + RESET + String.format(" (%dms, %.2fMB)", runTimeMs, runMemMB));
                    continue;
                }

                String userOut = new String(p.getInputStream().readAllBytes()).replace("\r\n", "\n").trim();
                String stdOut = Files.readString(outFile.toPath()).replace("\r\n", "\n").trim();

                String metricsStr = String.format(" (%dms, %.2fMB)", runTimeMs, runMemMB);

                if (userOut.equals(stdOut)) {
                    System.out.println(GREEN + "AC" + RESET + metricsStr);
                } else {
                    System.out.println(RED + "WA" + RESET + metricsStr);
                    System.out.println("    -----------------------------------------");
                    System.out.println("    " + GREEN + "[标准输出]" + RESET + "\n    " + stdOut.replace("\n", "\n    "));
                    System.out.println("    " + RED + "[你的输出]" + RESET + "\n    " + userOut.replace("\n", "\n    "));
                    System.out.println("    -----------------------------------------");
                }
            } catch (Exception e) {
                System.out.println(RED + "RE (System Error)" + RESET);
            } finally {
                try { Files.deleteIfExists(timeTmpPath); } catch (Exception ignored) {}
            }
        }
    }
}
