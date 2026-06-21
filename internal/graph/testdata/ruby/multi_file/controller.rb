class Api::V1::UsersController < ApplicationController
  def index
    users = User.where(active: true)
    render json: users
  end

  def create
    user = User.new(user_params)
    PaymentService.new.process(user)
    render json: user
  end

  private

  def user_params
    params.require(:user).permit(:name, :email)
  end
end
