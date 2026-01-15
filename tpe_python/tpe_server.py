import optuna
from flask import Flask, jsonify, request
import pandas as pd
import os
import threading

STUDY_NAME = "go_system_optimization"
DB_FILE = "db.sqlite3"
STORAGE = f"sqlite:///{DB_FILE}"
CSV_FILE = "optuna_results.csv"

app = Flask(__name__)

MIN_K = 1e-5
MAX_K = 1.0


if os.path.exists(DB_FILE):
    try:
        os.remove(DB_FILE)
        print(f"已删除旧数据库文件: {DB_FILE}")
    except PermissionError:
        print(f"无法删除 {DB_FILE}，请确保没有其他程序正在占用该文件。")

if os.path.exists(CSV_FILE):
    try:
        os.remove(CSV_FILE)
        print(f"已删除旧CSV文件: {CSV_FILE}")
    except OSError:
        pass

study = optuna.create_study(
    study_name=STUDY_NAME,
    storage=STORAGE,
    direction="maximize",
    load_if_exists=False,
    sampler=optuna.samplers.TPESampler()
)

current_trial = None
lock = threading.Lock()

def save_visualizations_and_csv():
    """
    保存CSV数据和可视化图表
    """
    df = study.trials_dataframe()
    df.to_csv(CSV_FILE, index=False)
    print(f"Data saved to {CSV_FILE}")

    try:
        fig_slice = optuna.visualization.plot_slice(study, params=["k"])
        fig_slice.write_html("vis_slice_k_vs_rate.html")

        fig_history = optuna.visualization.plot_optimization_history(study)
        fig_history.write_html("vis_history.html")
    except Exception as e:
        print(f"Visualization error: {e}")

@app.route('/init_k', methods=['GET'])
def get_init_k():
    global current_trial
    with lock:
        current_trial = study.ask()

        k_val = current_trial.suggest_float("k", MIN_K, MAX_K, log=True)

    return jsonify({"k": k_val, "trial_number": current_trial.number})

@app.route('/report_and_next', methods=['POST'])
def report_and_next():
    global current_trial
    data = request.json
    hit_rate = data.get("hit_rate")

    if current_trial is None:
        return jsonify({"error": "No active trial"}), 400

    with lock:
        print(f"Trial {current_trial.number} finished with hit_rate: {hit_rate}")
        study.tell(current_trial, hit_rate)

        save_visualizations_and_csv()

        current_trial = study.ask()

        next_k = current_trial.suggest_float("k", MIN_K, MAX_K, log=True)

    return jsonify({"k": next_k, "trial_number": current_trial.number})

if __name__ == '__main__':
    print(f"Starting Optuna TPE Server (Log-Uniform Mode: {MIN_K} - {MAX_K})...")
    app.run(host='0.0.0.0', port=15000, debug=False)